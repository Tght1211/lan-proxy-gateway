package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const gatewayHTTPUserAgent = "lan-proxy-gateway"

func newGatewayHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = false

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		req.Header.Set("User-Agent", gatewayHTTPUserAgent)
		req.Header.Set("Accept", "*/*")
		return nil
	}

	return client
}

func newGatewayHTTPRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", gatewayHTTPUserAgent)
	req.Header.Set("Accept", "*/*")
	return req, nil
}

func openGatewayURL(client *http.Client, url string) (*http.Response, error) {
	req, err := newGatewayHTTPRequest(http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func downloadGatewayURLToFile(client *http.Client, url, dest string) error {
	resp, err := openGatewayURL(client, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = os.Remove(dest)
		return err
	}

	return nil
}

func downloadGatewayURLToFileWithWindowsFallback(url, dest string, timeout time.Duration) error {
	client := newGatewayHTTPClient(timeout)
	httpErr := downloadGatewayURLToFile(client, url, dest)
	if httpErr == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		return httpErr
	}

	psErr := downloadWithPowerShell(url, dest)
	if psErr != nil {
		return fmt.Errorf("%v; PowerShell fallback failed: %w", httpErr, psErr)
	}
	return nil
}

func downloadWithPowerShell(url, dest string) error {
	script := fmt.Sprintf(
		"$ProgressPreference='SilentlyContinue'; Invoke-WebRequest -Uri %s -OutFile %s -Headers @{'User-Agent'='%s'} -UseBasicParsing",
		powerShellSingleQuote(url),
		powerShellSingleQuote(dest),
		gatewayHTTPUserAgent,
	)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(dest)
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}

	return nil
}

func powerShellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
