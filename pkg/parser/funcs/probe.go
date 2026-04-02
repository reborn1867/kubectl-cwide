package funcs

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/client-go/rest"
)

// NewProbeCheckFunction returns a template function that pings a pod's probe endpoint.
//
// Usage in templates:
//
//	{{ probeCheck . "readiness" }}
//	{{ probeCheck . "liveness" }}
//	{{ probeCheck . "startup" }}
func NewProbeCheckFunction(config *rest.Config) func(obj interface{}, probeType string) string {
	return func(obj interface{}, probeType string) string {
		m, ok := obj.(map[string]interface{})
		if !ok {
			return "ERR (not a map)"
		}

		namespace := nestedString(m, "metadata", "namespace")
		podName := nestedString(m, "metadata", "name")
		if namespace == "" || podName == "" {
			return "ERR (no pod identity)"
		}

		probe := extractProbe(m, probeType)
		if probe == nil {
			return "N/A"
		}

		if _, ok := probe["exec"]; ok {
			return "N/A (exec)"
		}
		if _, ok := probe["grpc"]; ok {
			return "N/A (grpc)"
		}

		if httpGet, ok := probe["httpGet"].(map[string]interface{}); ok {
			return checkHTTPProbe(config, namespace, podName, httpGet)
		}

		if tcpSocket, ok := probe["tcpSocket"].(map[string]interface{}); ok {
			return checkTCPProbe(config, namespace, podName, tcpSocket)
		}

		return "N/A (unknown)"
	}
}

// extractProbe navigates the unstructured pod to find the probe spec.
func extractProbe(pod map[string]interface{}, probeType string) map[string]interface{} {
	var probeField string
	switch strings.ToLower(probeType) {
	case "readiness":
		probeField = "readinessProbe"
	case "liveness":
		probeField = "livenessProbe"
	case "startup":
		probeField = "startupProbe"
	default:
		return nil
	}

	containers, ok := nestedSlice(pod, "spec", "containers")
	if !ok || len(containers) == 0 {
		return nil
	}

	// Check the first container
	container, ok := containers[0].(map[string]interface{})
	if !ok {
		return nil
	}

	probe, ok := container[probeField].(map[string]interface{})
	if !ok {
		return nil
	}
	return probe
}

// checkHTTPProbe uses the Kubernetes API server pod proxy to reach an httpGet probe.
func checkHTTPProbe(config *rest.Config, namespace, podName string, httpGet map[string]interface{}) string {
	path, _ := httpGet["path"].(string)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	port := httpGet["port"]
	scheme := strings.ToLower(fmt.Sprintf("%v", httpGet["scheme"]))
	if scheme == "" || scheme == "<nil>" {
		scheme = "http"
	}

	// Build the API server proxy URL:
	// /api/v1/namespaces/{ns}/pods/{scheme}:{name}:{port}/proxy{path}
	proxyPath := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s:%s:%v/proxy%s",
		namespace, scheme, podName, port, path)

	transport, err := rest.TransportFor(config)
	if err != nil {
		return fmt.Sprintf("ERR (%s)", err)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	host := config.Host
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	url := host + proxyPath

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return fmt.Sprintf("ERR (%s)", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("FAIL (%s)", truncateError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return fmt.Sprintf("OK (%d)", resp.StatusCode)
	}
	return fmt.Sprintf("FAIL (%d)", resp.StatusCode)
}

// checkTCPProbe uses the Kubernetes API server pod proxy to check TCP connectivity.
func checkTCPProbe(config *rest.Config, namespace, podName string, tcpSocket map[string]interface{}) string {
	port := tcpSocket["port"]

	// Use the API server proxy — if the port is open, the proxy succeeds.
	// /api/v1/namespaces/{ns}/pods/{name}:{port}/proxy/
	proxyPath := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s:%v/proxy/",
		namespace, podName, port)

	transport, err := rest.TransportFor(config)
	if err != nil {
		return fmt.Sprintf("ERR (%s)", err)
	}

	// For TCP checks, we skip TLS verification on the proxy response since
	// we just care about connectivity.
	if t, ok := transport.(*http.Transport); ok {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	host := config.Host
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	url := host + proxyPath

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return fmt.Sprintf("ERR (%s)", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("FAIL (%s)", truncateError(err))
	}
	defer resp.Body.Close()

	// Any response (even 4xx) means the TCP port is reachable
	return "OK"
}

// nestedString safely extracts a string from a nested map.
func nestedString(m map[string]interface{}, keys ...string) string {
	for i, key := range keys {
		if i == len(keys)-1 {
			v, _ := m[key].(string)
			return v
		}
		next, ok := m[key].(map[string]interface{})
		if !ok {
			return ""
		}
		m = next
	}
	return ""
}

// nestedSlice safely extracts a slice from a nested map.
func nestedSlice(m map[string]interface{}, keys ...string) ([]interface{}, bool) {
	for i, key := range keys {
		if i == len(keys)-1 {
			v, ok := m[key].([]interface{})
			return v, ok
		}
		next, ok := m[key].(map[string]interface{})
		if !ok {
			return nil, false
		}
		m = next
	}
	return nil, false
}

// truncateError shortens error messages for display.
func truncateError(err error) string {
	s := err.Error()
	if len(s) > 40 {
		return s[:37] + "..."
	}
	return s
}
