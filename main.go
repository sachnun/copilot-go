package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"internal/app"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var err error
	switch cmd {
	case "start":
		err = runStart(ctx, args)
	case "auth":
		err = runAuth(ctx, args)
	case "check-usage":
		err = app.RunCheckUsage(ctx)
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runStart(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("start", flag.ExitOnError)

	port := fs.Int("port", 4141, "Port to listen on")
	fs.IntVar(port, "p", 4141, "Port to listen on")

	verbose := fs.Bool("verbose", false, "Enable verbose logging")
	fs.BoolVar(verbose, "v", false, "Enable verbose logging")

	accountType := fs.String("account-type", "individual", "Account type to use (individual, business, enterprise)")
	fs.StringVar(accountType, "a", "individual", "Account type to use")

	manual := fs.Bool("manual", false, "Enable manual request approval")

	rateLimitRaw := fs.String("rate-limit", "", "Rate limit in seconds between requests")
	fs.StringVar(rateLimitRaw, "r", "", "Rate limit in seconds between requests")

	waitFlag := fs.Bool("wait", false, "Wait instead of error when rate limit is hit")
	fs.BoolVar(waitFlag, "w", false, "Wait instead of error when rate limit is hit")

	githubToken := fs.String("github-token", "", "Provide GitHub token directly")
	fs.StringVar(githubToken, "g", "", "Provide GitHub token directly")

	claudeCode := fs.Bool("claude-code", false, "Generate Claude Code command (not supported in Go version)")

	showToken := fs.Bool("show-token", false, "Show GitHub and Copilot tokens on fetch and refresh")

	proxyEnv := fs.Bool("proxy-env", false, "Initialize proxy from environment variables")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *claudeCode {
		fmt.Println("[info] Claude Code command generation is not implemented in the Go port.")
	}

	var rateLimitPtr *int
	if *rateLimitRaw != "" {
		value, err := strconv.Atoi(*rateLimitRaw)
		if err != nil {
			return fmt.Errorf("invalid value for --rate-limit: %w", err)
		}
		rateLimitPtr = &value
	}

	return app.RunServer(ctx, app.RunServerOptions{
		Port:             *port,
		Verbose:          *verbose,
		AccountType:      *accountType,
		Manual:           *manual,
		RateLimitSeconds: rateLimitPtr,
		RateLimitWait:    *waitFlag,
		GitHubToken:      *githubToken,
		ShowToken:        *showToken,
		ProxyEnv:         *proxyEnv,
	})
}

func runAuth(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("auth", flag.ExitOnError)

	verbose := fs.Bool("verbose", false, "Enable verbose logging")
	fs.BoolVar(verbose, "v", false, "Enable verbose logging")

	showToken := fs.Bool("show-token", false, "Show GitHub token on auth")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return app.RunAuth(ctx, app.RunAuthOptions{
		Verbose:   *verbose,
		ShowToken: *showToken,
	})
}

func usage() {
	fmt.Println("Usage: copilot-api <command> [options]\n")
	fmt.Println("Commands:")
	fmt.Println("  start         Start the Copilot API server")
	fmt.Println("  auth          Run GitHub auth flow without running the server")
	fmt.Println("  check-usage   Show current GitHub Copilot usage/quota information")
}
