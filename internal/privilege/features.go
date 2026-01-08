package privilege

// FeaturesFromPortage converts Portage FEATURES to privilege Features.
//
// Supported Portage features:
//   - userpriv: Drop privileges for build phases
//   - userfetch: Drop privileges for fetch operations
//   - usersandbox: Enable user namespace sandbox
//
// Example:
//
//	portageFeatures := []string{"sandbox", "userpriv", "userfetch", "network-sandbox"}
//	features := privilege.FeaturesFromPortage(portageFeatures)
//	mgr, err := privilege.NewManager(features)
func FeaturesFromPortage(portageFeatures []string) Features {
	var f Features

	for _, feature := range portageFeatures {
		switch feature {
		case "userpriv":
			f.UserPriv = true
		case "userfetch":
			f.UserFetch = true
		case "usersandbox":
			f.UserSandbox = true
		}
	}

	return f
}

// DefaultFeatures returns the default Gentoo privilege features.
//
// By default, Gentoo enables:
//   - userpriv: Build phases run as portage user
//   - userfetch: Fetch operations run as portage user (less common)
//
// These match the default FEATURES in make.conf.
func DefaultFeatures() Features {
	return Features{
		UserPriv:    true,
		UserFetch:   true,
		UserSandbox: false, // Requires kernel support
	}
}

// StrictFeatures returns a strict security configuration.
//
// All privilege dropping features are enabled for maximum security.
// Requires kernel support for user namespaces.
func StrictFeatures() Features {
	return Features{
		UserPriv:    true,
		UserFetch:   true,
		UserSandbox: true,
	}
}

// NoopFeatures returns features with all privilege dropping disabled.
//
// This is useful for testing or when running builds that require
// root privileges throughout (rare).
func NoopFeatures() Features {
	return Features{
		UserPriv:    false,
		UserFetch:   false,
		UserSandbox: false,
	}
}

// HasPrivilegeFeature checks if the Portage features include userpriv.
func HasPrivilegeFeature(portageFeatures []string) bool {
	for _, f := range portageFeatures {
		if f == "userpriv" {
			return true
		}
	}
	return false
}

// HasFetchFeature checks if the Portage features include userfetch.
func HasFetchFeature(portageFeatures []string) bool {
	for _, f := range portageFeatures {
		if f == "userfetch" {
			return true
		}
	}
	return false
}

// AllPhases returns a list of all known ebuild phases.
//
// This is useful for iterating over phases to check privileges.
func AllPhases() []string {
	return []string{
		"fetch",
		"unpack",
		"prepare",
		"configure",
		"compile",
		"test",
		"install",
		"preinst",
		"postinst",
		"prerm",
		"postrm",
		"qmerge",
	}
}

// BuildPhases returns a list of build phases that can drop privileges.
func BuildPhases() []string {
	return []string{
		"fetch",
		"unpack",
		"prepare",
		"configure",
		"compile",
		"test",
		"install",
	}
}

// MergePhases returns a list of merge phases that require root.
func MergePhases() []string {
	return []string{
		"preinst",
		"postinst",
		"prerm",
		"postrm",
		"qmerge",
	}
}
