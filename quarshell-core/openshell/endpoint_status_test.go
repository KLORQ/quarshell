package openshell

import "testing"

func TestEndpointStatusIdentityMatchesPinnedVectors(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		ports  []uint32
		port   uint32
		path   string
		id     string
		host2  string
		path2  string
		ports2 []uint32
	}{
		{
			name: "multiple ports", host: "MCP.Example.COM", ports: []uint32{8080, 443}, path: "/**",
			id:    "endpoint:v1:97909e9ba498b0585100b47d41cf340faf0e8dd44e6f648dcebe5b25c35254cf",
			host2: "mcp.example.com", path2: "/**", ports2: []uint32{443, 8080},
		},
		{
			name: "legacy port and path", host: "tools.example.com", port: 8443, path: "/rpc",
			id:    "endpoint:v1:9d1ce7581eed8c962df9fe9b9e260b2df73ca71497c55cbe4c9850cb594c5837",
			host2: "tools.example.com", path2: "/rpc", ports2: []uint32{8443},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity := NewEndpointStatusIdentity(tt.host, tt.ports, tt.port, tt.path)
			if identity.ID != tt.id || identity.Host != tt.host2 || identity.Path != tt.path2 || len(identity.Ports) != len(tt.ports2) {
				t.Fatalf("identity=%+v; want id=%s host=%s path=%s ports=%v", identity, tt.id, tt.host2, tt.path2, tt.ports2)
			}
			for i := range tt.ports2 {
				if identity.Ports[i] != tt.ports2[i] {
					t.Fatalf("ports=%v; want %v", identity.Ports, tt.ports2)
				}
			}
		})
	}
}
