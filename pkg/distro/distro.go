package distro

import (
	"log"
	"os"
	"regexp"
)

// Distribution types
const (
	Unknown = iota
	Debian
	RedHat
)

// Distribution represents details about the Linux distribution
type Distribution struct {
	Type       int
	Name       string
	Version    string
	PackageCmd string
}

// Detect identifies the Linux distribution from system files
func Detect() (*Distribution, error) {
	dist := &Distribution{}

	// Check if /etc/os-release exists
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, err
	}

	// Parse the os-release file
	osRelease := string(data)
	nameRe := regexp.MustCompile(`ID="?([^"\n]+)"?`)
	versionRe := regexp.MustCompile(`VERSION_ID="?([^"\n]+)"?`)

	log.Printf("Detected OS release file: %s", nameRe)

	if nameMatch := nameRe.FindStringSubmatch(osRelease); len(nameMatch) > 1 {
		dist.Name = nameMatch[1]
		log.Printf("Detected OS release file: %s", string(osRelease))
		// Set distribution type and package command
		switch dist.Name {
		case "ubuntu", "debian":
			dist.Type = Debian
			dist.PackageCmd = "apt-get"
		case "centos", "rhel", "fedora":
			dist.Type = RedHat
			dist.PackageCmd = "yum"
		default:
			dist.Type = Unknown
		}
	}

	if versionMatch := versionRe.FindStringSubmatch(osRelease); len(versionMatch) > 1 {
		dist.Version = versionMatch[1]
	}

	return dist, nil
}

// IsDebian returns true if the distribution is Debian-based
func (d *Distribution) IsDebian() bool {
	return d.Type == Debian
}

// IsRedHat returns true if the distribution is RedHat-based
func (d *Distribution) IsRedHat() bool {
	return d.Type == RedHat
}
