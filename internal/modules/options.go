package modules

// Options holds immutable per-run module settings.
type Options struct {
	Colors   ColorsOptions
	Disk     DiskOptions
	Time     TimeOptions
	Battery  BatteryOptions
	LocalIP  LocalIPOptions
	Packages PackagesOptions
}

// ColorsOptions selects the palette swatch shape.
type ColorsOptions struct {
	Shape string `toml:"shape"`
}

// DiskOptions selects a mount.
type DiskOptions struct {
	Mount string `toml:"mount"`
}

// TimeOptions selects a Go time layout.
type TimeOptions struct {
	Format string `toml:"format"`
}

// BatteryOptions selects a power supply entry.
type BatteryOptions struct {
	Device string `toml:"device"`
}

// LocalIPOptions selects an interface and address family.
type LocalIPOptions struct {
	Interface string `toml:"interface"`
	Family    string `toml:"family"`
}

// PackagesOptions selects a package database.
type PackagesOptions struct {
	Manager string `toml:"manager"`
}

// DefaultOptions returns independent default settings.
func DefaultOptions() Options {
	return Options{Colors: ColorsOptions{Shape: "blocks"}, Disk: DiskOptions{Mount: "/"}, Time: TimeOptions{Format: "15:04"}, LocalIP: LocalIPOptions{Family: "ipv4"}, Packages: PackagesOptions{Manager: "auto"}}
}
func options(env Env) Options {
	if env.Options != nil {
		return *env.Options
	}
	return DefaultOptions()
}

// DiskStats distinguishes all free bytes from unprivileged availability.
type DiskStats struct{ Total, Free, Available uint64 }
