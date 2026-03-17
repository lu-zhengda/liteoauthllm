package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/lu-zhengda/liteoauthllm/internal/auth"
	"github.com/lu-zhengda/liteoauthllm/internal/config"
	"github.com/lu-zhengda/liteoauthllm/internal/provider"
	"github.com/lu-zhengda/liteoauthllm/internal/proxy"
)

const (
	version = "0.1.0"

	// openaiClientID is the public OAuth client ID for PKCE-based CLI authentication.
	// Public client IDs are not secrets — they are required in OAuth authorization
	// requests and are visible to anyone inspecting the authorization URL.
	openaiClientID  = "app_EMoamEEZ73f0CkXaXp7hrann"
	openaiAuthURL   = "https://auth.openai.com/oauth/authorize"
	openaiTokenURL  = "https://auth.openai.com/oauth/token"
	openaiScopes    = "openid+profile+email+offline_access"
	openaiCallback  = "http://localhost:1455/auth/callback"
	openaiCallbackPort = 1455
)

func main() {
	if len(os.Args) < 2 {
		runServe(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "login":
		runLogin(os.Args[2:])
	case "logout":
		runLogout(os.Args[2:])
	case "status":
		runStatus()
	case "serve":
		runServe(os.Args[2:])
	case "version":
		fmt.Printf("liteoauthllm v%s\n", version)
	default:
		runServe(os.Args[1:])
	}
}

func tokenDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".liteoauthllm", "tokens")
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".liteoauthllm")
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 0, "port to listen on")
	verbose := fs.Bool("v", false, "verbose request logging")
	configPath := fs.String("config", "", "path to config file")
	fs.Parse(args) //nolint:errcheck

	cfg := config.Default()

	if *configPath != "" {
		fileCfg, err := config.LoadFile(*configPath)
		if err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
		cfg = config.Merge(cfg, fileCfg)
	} else {
		defaultPath := filepath.Join(configDir(), "config.yaml")
		if fileCfg, err := config.LoadFile(defaultPath); err == nil {
			cfg = config.Merge(cfg, fileCfg)
		}
	}

	flagCfg := config.Config{}
	if *port != 0 {
		flagCfg.Port = *port
	}
	if *verbose {
		flagCfg.Verbose = true
	}
	cfg = config.Merge(cfg, flagCfg)

	store := auth.NewStore(tokenDir())
	reg := provider.NewRegistry()
	srv := proxy.NewServer(reg, store, cfg.Verbose)

	fmt.Printf("liteoauthllm v%s\n", version)
	printProviderStatus(store)
	fmt.Printf("  listening on http://127.0.0.1:%d\n\n", cfg.Port)
	fmt.Printf("  OPENAI_BASE_URL=http://127.0.0.1:%d/v1\n", cfg.Port)
	fmt.Printf("  ANTHROPIC_BASE_URL=http://127.0.0.1:%d\n\n", cfg.Port)

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", cfg.Port),
		Handler: srv,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	fmt.Println("\nshutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
}

func runLogin(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: liteoauthllm login <openai|anthropic>")
		os.Exit(1)
	}

	store := auth.NewStore(tokenDir())
	providerName := args[0]

	switch providerName {
	case "openai":
		loginOpenAI(store)
	case "anthropic":
		loginAnthropic(store)
	default:
		fmt.Fprintf(os.Stderr, "unknown provider: %s (supported: openai, anthropic)\n", providerName)
		os.Exit(1)
	}
}

func loginOpenAI(store *auth.Store) {
	verifier, challenge, err := auth.GeneratePKCE()
	if err != nil {
		log.Fatalf("Failed to generate PKCE: %v", err)
	}

	state, err := auth.GenerateState()
	if err != nil {
		log.Fatalf("Failed to generate state: %v", err)
	}

	authURL := fmt.Sprintf(
		"%s?client_id=%s&scope=%s&redirect_uri=%s&code_challenge=%s&code_challenge_method=S256&response_type=code&state=%s",
		openaiAuthURL,
		openaiClientID,
		openaiScopes,
		openaiCallback,
		challenge,
		state,
	)

	fmt.Println("Opening browser for OpenAI login...")
	openBrowser(authURL)

	code, err := auth.WaitForCallback(openaiCallbackPort, state)
	if err != nil {
		log.Fatalf("OAuth callback failed: %v", err)
	}

	token, err := auth.ExchangeCode(
		openaiTokenURL,
		openaiClientID,
		code,
		verifier,
		openaiCallback,
	)
	if err != nil {
		log.Fatalf("Token exchange failed: %v", err)
	}

	if err := store.Write("openai", token); err != nil {
		log.Fatalf("Failed to save token: %v", err)
	}

	fmt.Println("  ✓ OpenAI login successful")
}

func loginAnthropic(store *auth.Store) {
	fmt.Println("Anthropic uses setup-tokens for authentication.")
	fmt.Println("Run `claude setup-token` in your terminal to generate a token, then paste it here.")
	fmt.Println()
	fmt.Print("Token: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	tokenStr := strings.TrimSpace(scanner.Text())

	if !strings.HasPrefix(tokenStr, "sk-ant-oat01-") || len(tokenStr) < 80 {
		fmt.Fprintln(os.Stderr, "Invalid token. Must start with 'sk-ant-oat01-' and be at least 80 characters.")
		os.Exit(1)
	}

	token := auth.Token{
		Version:     1,
		AccessToken: tokenStr,
	}

	if err := store.Write("anthropic", token); err != nil {
		log.Fatalf("Failed to save token: %v", err)
	}

	fmt.Println("  ✓ Anthropic setup-token saved")
}

func runLogout(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: liteoauthllm logout <openai|anthropic>")
		os.Exit(1)
	}

	store := auth.NewStore(tokenDir())
	providerName := args[0]

	if err := store.Delete(providerName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  ✓ %s token removed\n", providerName)
}

func runStatus() {
	store := auth.NewStore(tokenDir())
	fmt.Println("liteoauthllm status")
	printProviderStatus(store)
}

func printProviderStatus(store *auth.Store) {
	for _, name := range []string{"openai", "anthropic"} {
		token, err := store.Read(name)
		if err != nil {
			fmt.Printf("  ✗ %-10s (not configured)\n", name)
			continue
		}

		if token.ExpiresAt == 0 {
			fmt.Printf("  ✓ %-10s (setup-token configured)\n", name)
		} else if auth.NeedsRefresh(token) {
			fmt.Printf("  ✗ %-10s (token expired)\n", name)
		} else {
			remaining := time.Until(time.Unix(token.ExpiresAt, 0)).Round(time.Hour)
			fmt.Printf("  ✓ %-10s (token valid, expires in %s)\n", name, remaining)
		}
	}
}

func openBrowser(rawURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	}
	if cmd != nil {
		if err := cmd.Start(); err != nil {
			fmt.Printf("Could not open browser. Please navigate to:\n%s\n", rawURL)
		}
	} else {
		fmt.Printf("Could not open browser. Please navigate to:\n%s\n", rawURL)
	}
}
