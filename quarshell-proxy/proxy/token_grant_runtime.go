package proxy

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	openshellv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	maxCachedGrantTokens     = 256
	maxGrantAccessTokenBytes = 64 << 10
)

// SPIFFETokenGrantResolver performs workload SVID acquisition, the gateway's
// subject-token exchange, and the final RFC 8693 exchange. It is process-local
// to a sandbox supervisor and keeps only short-lived access tokens in memory.
type SPIFFETokenGrantResolver struct {
	mu    sync.Mutex
	cache map[string]cachedGrantToken
	Now   func() time.Time
}

type cachedGrantToken struct {
	value   string
	expires time.Time
}

func NewSPIFFETokenGrantResolver() *SPIFFETokenGrantResolver {
	return &SPIFFETokenGrantResolver{cache: make(map[string]cachedGrantToken)}
}

func (r *SPIFFETokenGrantResolver) ResolveTokenGrant(ctx context.Context, g TokenGrantCredential, target TokenGrantTarget) (string, error) {
	grantType := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(g.GrantType), "-", "_"))
	if strings.TrimSpace(g.TokenEndpoint) == "" || (grantType != "token_exchange" && grantType != "client_credentials") {
		return "", fmt.Errorf("unsupported or incomplete token grant")
	}
	audience, scopes := tokenGrantTarget(g, target)
	cacheKey, err := tokenGrantCacheKey(g, target)
	if err != nil {
		return "", fmt.Errorf("encode token grant cache identity: %w", err)
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	r.mu.Lock()
	if cached, ok := r.cache[cacheKey]; ok && now.Before(cached.expires) {
		r.mu.Unlock()
		return cached.value, nil
	}
	delete(r.cache, cacheKey)
	r.mu.Unlock()

	jwtAudience := strings.TrimSpace(g.JWTSVIDAudience)
	if jwtAudience == "" {
		jwtAudience = deriveJWTGrantAudience(g.TokenEndpoint)
	}
	socket := firstTokenGrantEnv("OPENSHELL_PROVIDER_SPIFFE_WORKLOAD_API_SOCKET", "QUARSHELL_PROVIDER_SPIFFE_WORKLOAD_API_SOCKET")
	if socket == "" {
		return "", fmt.Errorf("SPIFFE Workload API socket is not configured")
	}
	endpoint, err := spiffeWorkloadAPIAddress(socket)
	if err != nil {
		return "", err
	}
	source, err := workloadapi.NewJWTSource(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(endpoint)))
	if err != nil {
		return "", fmt.Errorf("SPIFFE Workload API unavailable")
	}
	defer source.Close()
	svid, err := source.FetchJWTSVID(ctx, jwtsvid.Params{Audience: jwtAudience})
	if err != nil {
		return "", fmt.Errorf("JWT-SVID unavailable")
	}
	if _, err := jwtsvid.ParseAndValidate(svid.Marshal(), source, []string{jwtAudience}); err != nil {
		return "", fmt.Errorf("gateway JWT-SVID validation failed")
	}

	var subjectToken string
	if grantType == "token_exchange" {
		subjectToken, err = exchangeAtGateway(ctx, g, svid.Marshal())
		if err != nil {
			return "", err
		}
	}
	form := url.Values{}
	if grantType == "client_credentials" {
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
		form.Set("assertion", svid.Marshal())
	} else {
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
		form.Set("client_assertion_type", firstNonEmptyToken(g.ClientAssertionType, "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"))
		form.Set("client_assertion", svid.Marshal())
		form.Set("subject_token", subjectToken)
		form.Set("subject_token_type", firstNonEmptyToken(g.SubjectTokenType, "urn:ietf:params:oauth:token-type:access_token"))
		form.Set("requested_token_type", firstNonEmptyToken(g.RequestedTokenType, "urn:ietf:params:oauth:token-type:access_token"))
	}
	if audience != "" {
		form.Set("audience", audience)
	}
	if len(scopes) > 0 {
		form.Set("scope", strings.Join(scopes, " "))
	}
	token, err := postTokenGrant(ctx, g.TokenEndpoint, form)
	if err != nil {
		return "", err
	}
	// An explicit cache TTL may shorten, but must never outlive, the provider token.
	ttlSeconds := token.ExpiresIn
	if g.CacheTTLSeconds > 0 && (ttlSeconds <= 0 || g.CacheTTLSeconds < ttlSeconds) {
		ttlSeconds = g.CacheTTLSeconds
	}
	const maxDurationSeconds = int64((1<<63 - 1) / int64(time.Second))
	if ttlSeconds > maxDurationSeconds {
		return "", fmt.Errorf("token endpoint returned an invalid expiry")
	}
	ttl := time.Duration(ttlSeconds) * time.Second
	boundedByProviderExpiry := token.ExpiresIn > 0 && ttlSeconds >= token.ExpiresIn
	if boundedByProviderExpiry {
		ttl -= 30 * time.Second
	}
	if boundedByProviderExpiry && ttl < time.Second {
		ttl = time.Second
	} else if ttl <= 0 {
		ttl = 30 * time.Second
	} else if ttl < time.Second {
		ttl = time.Second
	}
	r.mu.Lock()
	result := r.cacheTokenLocked(now, cacheKey, cachedGrantToken{token.AccessToken, now.Add(ttl)})
	r.mu.Unlock()
	return result, nil
}

// cacheTokenLocked keeps the in-memory credential cache bounded. Expired
// entries are discarded first; when every entry is live, the token expiring
// soonest is evicted because it can be reacquired from the provider.
func tokenGrantCacheKey(g TokenGrantCredential, target TokenGrantTarget) (string, error) {
	cacheMaterial, err := json.Marshal(struct {
		Credential TokenGrantCredential `json:"credential"`
		Target     TokenGrantTarget     `json:"target"`
	}{Credential: g, Target: target})
	if err != nil {
		return "", err
	}
	cacheDigest := sha256.Sum256(cacheMaterial)
	return hex.EncodeToString(cacheDigest[:]), nil
}

func (r *SPIFFETokenGrantResolver) cacheTokenLocked(now time.Time, key string, candidate cachedGrantToken) string {
	if r.cache == nil {
		r.cache = make(map[string]cachedGrantToken)
	}
	if current, ok := r.cache[key]; ok && now.Before(current.expires) && current.expires.After(candidate.expires) {
		return current.value
	}
	for cacheKey, cached := range r.cache {
		if !now.Before(cached.expires) {
			delete(r.cache, cacheKey)
		}
	}
	if _, exists := r.cache[key]; !exists && len(r.cache) >= maxCachedGrantTokens {
		var earliestKey string
		var earliest time.Time
		for cacheKey, cached := range r.cache {
			if earliestKey == "" || cached.expires.Before(earliest) {
				earliestKey, earliest = cacheKey, cached.expires
			}
		}
		delete(r.cache, earliestKey)
	}
	r.cache[key] = candidate
	return candidate.value
}

// spiffeWorkloadAPIAddress accepts the normal host UNIX socket path and the
// explicitly configured tcp:IP:port endpoint used by VM provider token grants.
// TCP is plaintext and is only supplied by Gateway after the operator's guest
// exposure acknowledgement has been validated.
func spiffeWorkloadAPIAddress(raw string) (string, error) {
	if strings.HasPrefix(raw, "tcp:") {
		address, err := netip.ParseAddrPort(strings.TrimPrefix(raw, "tcp:"))
		if err != nil || address.Port() == 0 || address.Addr().IsUnspecified() {
			return "", fmt.Errorf("SPIFFE Workload API TCP endpoint is invalid")
		}
		return "tcp://" + address.String(), nil
	}
	if strings.HasPrefix(raw, "unix://") {
		return raw, nil
	}
	if strings.HasPrefix(raw, "unix:") {
		return "unix://" + strings.TrimPrefix(raw, "unix:"), nil
	}
	if filepath.IsAbs(raw) {
		return "unix://" + raw, nil
	}
	return "", fmt.Errorf("SPIFFE Workload API endpoint must be an absolute UNIX socket path or acknowledged tcp:IP:port")
}

func exchangeAtGateway(ctx context.Context, g TokenGrantCredential, supervisorSVID string) (string, error) {
	base := firstTokenGrantEnv("QUARSHELL_GATEWAY_GRPC_ENDPOINT", "QUARSHELL_GATEWAY_URL", "OPENSHELL_GATEWAY")
	if base == "" {
		return "", fmt.Errorf("gateway URL is not configured")
	}
	if firstTokenGrantEnv("QUARSHELL_SANDBOX") == "" || firstTokenGrantEnv("QUARSHELL_SANDBOX_TOKEN") == "" || g.Provider == "" || g.CredentialKey == "" {
		return "", fmt.Errorf("sandbox token-grant identity is not configured")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("gateway URL is invalid")
	}
	if u.Port() == "" {
		if port := firstTokenGrantEnv("QUARSHELL_GATEWAY_GRPC_PORT"); port != "" {
			u.Host = net.JoinHostPort(u.Hostname(), port)
		}
	}
	var transport credentials.TransportCredentials
	if strings.EqualFold(u.Scheme, "https") {
		tlsConfig, err := tokenGrantTLSConfigFromEnvironment(u.Hostname())
		if err != nil {
			return "", err
		}
		transport = credentials.NewTLS(tlsConfig)
	} else if strings.EqualFold(u.Scheme, "http") {
		if firstTokenGrantEnv("QUARSHELL_GUEST_TLS_CA") != "" || firstTokenGrantEnv("QUARSHELL_GUEST_TLS_CERT") != "" || firstTokenGrantEnv("QUARSHELL_GUEST_TLS_KEY") != "" {
			return "", fmt.Errorf("guest TLS materials require an https gateway endpoint")
		}
		transport = insecure.NewCredentials()
	} else {
		return "", fmt.Errorf("gateway URL scheme is unsupported")
	}
	conn, err := grpc.NewClient(u.Host, grpc.WithTransportCredentials(transport))
	if err != nil {
		return "", fmt.Errorf("gateway RPC connection failed")
	}
	defer conn.Close()
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	callCtx = metadata.AppendToOutgoingContext(callCtx, "authorization", "Bearer "+firstTokenGrantEnv("QUARSHELL_SANDBOX_TOKEN"))
	resp, err := openshellv1.NewOpenShellClient(conn).ExchangeProviderSubjectToken(callCtx, &openshellv1.ExchangeProviderSubjectTokenRequest{
		SandboxId: firstTokenGrantEnv("QUARSHELL_SANDBOX"), Provider: g.Provider, CredentialKey: g.CredentialKey, SupervisorJwtSvid: supervisorSVID,
	})
	if err != nil || resp.GetAccessToken() == "" {
		return "", fmt.Errorf("gateway subject-token exchange failed")
	}
	return resp.GetAccessToken(), nil
}

func tokenGrantTLSConfigFromEnvironment(serverName string) (*tls.Config, error) {
	caPath := firstTokenGrantEnv("QUARSHELL_GUEST_TLS_CA")
	certPath := firstTokenGrantEnv("QUARSHELL_GUEST_TLS_CERT")
	keyPath := firstTokenGrantEnv("QUARSHELL_GUEST_TLS_KEY")
	if caPath == "" && certPath == "" && keyPath == "" {
		return &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}, nil
	}
	if caPath == "" || certPath == "" || keyPath == "" {
		return nil, fmt.Errorf("guest TLS CA, certificate and key must be configured together")
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read guest TLS CA bundle")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("guest TLS CA bundle contains no certificates")
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load guest TLS client certificate and key")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName, RootCAs: roots, Certificates: []tls.Certificate{cert}}, nil
}

func tokenGrantTarget(g TokenGrantCredential, target TokenGrantTarget) (string, []string) {
	audience, scopes := g.Audience, append([]string(nil), g.Scopes...)
	for _, override := range g.AudienceOverrides {
		if !strings.EqualFold(override.Host, target.Host) || (override.Port != 0 && override.Port != target.Port) {
			continue
		}
		if override.Path != "" && !tokenGrantPathMatches(override.Path, target.Path) {
			continue
		}
		if override.Audience != "" {
			audience = override.Audience
		}
		if len(override.Scopes) > 0 {
			scopes = append([]string(nil), override.Scopes...)
		}
		break
	}
	sort.Strings(scopes)
	return audience, scopes
}

func tokenGrantPathMatches(pattern, value string) bool {
	if pattern == value {
		return true
	}
	if strings.Contains(pattern, "**") {
		prefix := strings.SplitN(pattern, "**", 2)[0]
		return strings.HasPrefix(value, prefix)
	}
	matched, err := path.Match(pattern, value)
	return err == nil && matched
}

func deriveJWTGrantAudience(endpoint string) string {
	if i := strings.Index(endpoint, "/realms/"); i >= 0 {
		rest := endpoint[i+len("/realms/"):]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			return endpoint[:i+len("/realms/")+slash]
		}
	}
	return endpoint
}

func postTokenGrant(ctx context.Context, endpoint string, form url.Values) (struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}, error) {
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	u, parseErr := url.Parse(endpoint)
	if parseErr != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || !validTokenGrantEndpoint(u) {
		return token, fmt.Errorf("token endpoint is invalid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return token, fmt.Errorf("token endpoint is invalid")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return token, fmt.Errorf("token endpoint request failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return token, fmt.Errorf("token endpoint rejected grant (HTTP %d)", resp.StatusCode)
	}
	if json.Unmarshal(body, &token) != nil || token.AccessToken == "" || len(token.AccessToken) > maxGrantAccessTokenBytes {
		return token, fmt.Errorf("token endpoint returned an invalid response")
	}
	return token, nil
}

func validTokenGrantEndpoint(u *url.URL) bool {
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".svc") || strings.HasSuffix(host, ".svc.cluster.local") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func firstTokenGrantEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
func firstNonEmptyToken(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
