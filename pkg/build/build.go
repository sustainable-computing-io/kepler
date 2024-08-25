package build

var (
	// Version is the version of the exporter. Set by the linker flags in the Makefile.
	Version string
	// Revision is the Git commit that was compiled. Set by the linker flags in the Makefile.
	Revision string
	// Branch is the Git branch that was compiled. Set by the linker flags in the Makefile.
	Branch string
	// OS is the operating system the exporter was built for. Set by the linker flags in the Makefile.
	OS string
	// Arch is the architecture the exporter was built for. Set by the linker flags in the Makefile.
	Arch string
)
