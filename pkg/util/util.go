package util

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	ColorBlue   = "\033[0;34m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[0;33m"
	ColorReset  = "\033[0m"
)

// DisplayBanner displays the application banner
func DisplayBanner(appName string, version string) {
	bannerPath := "assets/banner.txt"
	bannerData, err := os.ReadFile(bannerPath)

	// Display ascii art if file exists, otherwise use simple banner
	if err == nil {
		fmt.Println(ColorBlue + string(bannerData) + ColorReset)
	} else {
		fmt.Printf(`
 ██ ▄█▀ █    ██  ▄▄▄▄   ▓█████  ▄████▄   ▒█████   ██▀███     ▄████ ▓█████ 
 ██▄█▒  ██  ▓██▒▓█████▄ ▓█   ▀ ▒██▀ ▀█  ▒██▒  ██▒▓██ ▒ ██▒  ██▒ ▀█▒▓█   ▀ 
▓███▄░ ▓██  ▒██░▒██▒ ▄██▒███   ▒▓█    ▄ ▒██░  ██▒▓██ ░▄█ ▒ ▒██░▄▄▄░▒███   
▓██ █▄ ▓▓█  ░██░▒██░█▀  ▒▓█  ▄ ▒▓▓▄ ▄██▒▒██   ██░▒██▀▀█▄   ░▓█  ██▓▒▓█  ▄ 
▒██▒ █▄▒▒█████▓ ░▓█  ▀█▓░▒████▒▒ ▓███▀ ░░ ████▓▒░░██▓ ▒██▒ ░▒▓███▀▒░▒████▒
▒ ▒▒ ▓▒░▒▓▒ ▒ ▒ ░▒▓███▀▒░░ ▒░ ░░ ░▒ ▒  ░░ ▒░▒░▒░ ░ ▒▓ ░▒▓░  ░▒   ▒ ░░ ▒░ ░
░ ░▒ ▒░░░▒░ ░ ░ ▒░▒   ░  ░ ░  ░  ░  ▒     ░ ▒ ▒░   ░▒ ░ ▒░   ░   ░  ░ ░  ░
░ ░░ ░  ░░░ ░ ░  ░    ░    ░   ░          ░ ░ ░ ▒    ░░   ░  ░ ░   ░    ░   
░  ░      ░      ░         ░  ░░ ░          ░ ░     ░            ░    ░  ░
                    ░         ░                                           
`)
	}

	fmt.Printf("%s%s v%s%s\n", ColorGreen, appName, version, ColorReset)
	fmt.Println("Crafting robust Kubernetes clusters with precision")
	fmt.Println("Copyright © 2025 - All Rights Reserved")
	fmt.Println()
}

// PromptWithDefault gets user input with a default value
func PromptWithDefault(prompt, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s (default: %s): ", prompt, defaultValue)
	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	return input
}

// PromptYesNo prompts for a yes/no answer
func PromptYesNo(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s (y/n): ", prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false
		}

		input = strings.ToLower(strings.TrimSpace(input))
		if input == "y" || input == "yes" {
			return true
		} else if input == "n" || input == "no" {
			return false
		}

		fmt.Println("Please enter 'y' or 'n'.")
	}
}

// GetDefaultIP returns the default IP address
func GetDefaultIP() string {
	cmd := exec.Command("hostname", "-I")
	output, err := cmd.Output()
	if err != nil {
		return "127.0.0.1"
	}

	// Use the first IP address
	ips := strings.Fields(string(output))
	if len(ips) > 0 {
		return ips[0]
	}

	return "127.0.0.1"
}
