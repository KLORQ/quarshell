package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) == 5 && os.Args[1] == "spawn-child" {
		spawnChild(os.Args[2], os.Args[3], os.Args[4])
		return
	}
	if len(os.Args) == 5 && os.Args[1] == "wait-child" {
		waitChild(os.Args[2], os.Args[3], os.Args[4])
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "spawn-orphans" {
		spawnOrphans(os.Args[2], os.Args[3])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "probe-tcp" {
		probeTCP(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "probe-udp" {
		probeUDP(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "probe-connect" {
		probeConnect(os.Args[2])
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "probe-connect-status" {
		probeConnectStatus(os.Args[2], os.Args[3])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "serve-tcp" {
		serveTCP(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "serve-udp" {
		serveUDP(os.Args[2])
		return
	}
	if len(os.Args) == 5 && os.Args[1] == "serve-proxy" {
		serveProxy(os.Args[2], os.Args[3], os.Args[4])
		return
	}
	addr := "127.0.0.1:17777"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	fmt.Println("READY")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		_ = listener.Close()
	}()

	var handlers sync.WaitGroup
	defer handlers.Wait()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Op == "accept" {
				return
			}
			log.Print(err)
			return
		}
		handlers.Add(1)
		go func() {
			defer handlers.Done()
			defer conn.Close()
			if _, err := io.Copy(conn, conn); err != nil {
				log.Print(err)
			}
		}()
	}
}

func spawnChild(pidFile, exitFile, seconds string) {
	child := exec.Command(os.Args[0], "wait-child", pidFile, exitFile, seconds)
	child.Stdin = strings.NewReader("")
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		log.Fatal(err)
	}
	if err := child.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		log.Fatal(err)
	}
}

func waitChild(pidFile, exitFile, seconds string) {
	delay, err := time.ParseDuration(seconds + "s")
	if err != nil {
		log.Fatalf("invalid child duration %q: %v", seconds, err)
	}
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		log.Fatal(err)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(signals)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case sig := <-signals:
		if err := os.WriteFile(exitFile, []byte(fmt.Sprintf("%d %s\n", os.Getpid(), sig)), 0o600); err != nil {
			log.Fatal(err)
		}
	case <-timer.C:
	}
}

func spawnOrphans(countArg, seconds string) {
	count, err := strconv.Atoi(countArg)
	if err != nil || count < 1 || count > 256 {
		log.Fatalf("invalid orphan count %q", countArg)
	}
	pids := make([]string, 0, count)
	for range count {
		child := exec.Command("sleep", seconds)
		child.Stdin = strings.NewReader("")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			log.Fatal(err)
		}
		pids = append(pids, strconv.Itoa(child.Process.Pid))
	}
	if err := os.WriteFile("/tmp/quarshell-process-event-child-pids", []byte(strings.Join(pids, "\n")), 0o600); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, "spawned %d adopted-child probes: %s\n", len(pids), strings.Join(pids, ","))
	os.Exit(0)
}

func serveTCP(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("READY tcp")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			defer conn.Close()
			_, _ = io.Copy(conn, conn)
		}()
	}
}

func serveUDP(addr string) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("READY udp")
	buf := make([]byte, 2048)
	for {
		n, peer, err := conn.ReadFrom(buf)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := conn.WriteTo(buf[:n], peer); err != nil {
			log.Print(err)
		}
	}
}

func probeTCP(target string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", target)
	if err != nil {
		fmt.Println("BLOCKED")
		return
	}
	_ = conn.Close()
	fmt.Fprintln(os.Stderr, "TCP probe connected to", target)
	os.Exit(2)
}

func probeUDP(target string) {
	conn, err := net.DialTimeout("udp", target, time.Second)
	if err != nil {
		fmt.Println("BLOCKED")
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte("quarshell-policy-bypass-probe")); err != nil {
		fmt.Println("BLOCKED")
		return
	}
	var response [1]byte
	if _, err := conn.Read(response[:]); err != nil {
		fmt.Println("BLOCKED")
		return
	}
	fmt.Fprintln(os.Stderr, "UDP probe received a response from", target)
	os.Exit(2)
}

func probeConnect(target string) {
	probeConnectStatus(target, "403")
}

func probeConnectStatus(target, expected string) {
	proxyAddr := strings.TrimSpace(os.Getenv("HTTPS_PROXY"))
	if proxyAddr == "" {
		fmt.Fprintln(os.Stderr, "HTTPS_PROXY is not configured for policy probe")
		os.Exit(3)
	}
	proxyURL, err := http.ProxyFromEnvironment(&http.Request{URL: &url.URL{Scheme: "https", Host: target}})
	if err != nil || proxyURL == nil {
		fmt.Fprintln(os.Stderr, "HTTPS_PROXY is not configured for policy probe")
		os.Exit(3)
	}
	proxyAuth := ""
	if authPath := strings.TrimSpace(os.Getenv("QUARSHELL_PROXY_AUTH_FILE")); authPath != "" {
		body, readErr := os.ReadFile(authPath)
		if readErr != nil || len(body) > 64*1024 {
			fmt.Fprintln(os.Stderr, "cannot read configured proxy auth file")
			os.Exit(3)
		}
		username, password, ok := strings.Cut(strings.TrimSpace(string(body)), ":")
		if !ok || strings.TrimSpace(username) == "" {
			fmt.Fprintln(os.Stderr, "configured proxy auth file must contain user:pass")
			os.Exit(3)
		}
		proxyAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	}
	conn, err := net.DialTimeout("tcp", proxyURL.Host, time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot reach configured HTTP proxy", proxyAddr, err)
		os.Exit(3)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n%s\r\n", target, target, proxyAuthorizationHeader(proxyAuth)); err != nil {
		fmt.Fprintln(os.Stderr, "write CONNECT request:", err)
		os.Exit(3)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		fmt.Fprintln(os.Stderr, "read CONNECT response:", err)
		os.Exit(3)
	}
	want, err := strconv.Atoi(expected)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid expected proxy status", expected)
		os.Exit(3)
	}
	if response.StatusCode != want {
		fmt.Fprintln(os.Stderr, "proxy CONNECT status", response.StatusCode, "for", target, "; want", want)
		os.Exit(2)
	}
	fmt.Println("PROXY", response.Status)
}

func proxyAuthorizationHeader(value string) string {
	if value == "" {
		return ""
	}
	return "Proxy-Authorization: " + value + "\r\n"
}

func serveProxy(addr, username, password string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("READY proxy")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleProxyConn(conn, username, password)
	}
}

func handleProxyConn(conn net.Conn, username, password string) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	request, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil || request.Method != http.MethodConnect {
		return
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	if request.Header.Get("Proxy-Authorization") != want {
		fmt.Printf("proxy auth mismatch got=%q want=%q\n", request.Header.Get("Proxy-Authorization"), want)
		_, _ = io.WriteString(conn, "HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic\r\n\r\n")
		return
	}
	upstream, err := net.DialTimeout("tcp", request.Host, 2*time.Second)
	if err != nil {
		_, _ = io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer upstream.Close()
	_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	go func() { _, _ = io.Copy(upstream, conn) }()
	_, _ = io.Copy(conn, upstream)
}
