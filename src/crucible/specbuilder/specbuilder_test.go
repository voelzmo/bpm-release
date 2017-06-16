package specbuilder_test

import (
	"crucible/config"
	"crucible/specbuilder"
	"crucible/specbuilder/specbuilderfakes"
	"errors"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Specbuilder", func() {
	var (
		cfg          *config.CrucibleConfig
		userIDFinder *specbuilderfakes.FakeUserIDFinder
		jobName      string
	)

	BeforeEach(func() {
		cfg = &config.CrucibleConfig{
			Process: &config.Process{
				Executable: "/var/vcap/packages/ambien/bin/ambien",
				Args: []string{
					"foo",
					"bar",
				},
				Env: []string{
					"RAVE=true",
					"ONE=two",
				},
			},
		}

		jobName = "ambien-job"

		userIDFinder = &specbuilderfakes.FakeUserIDFinder{}
		userIDFinder.LookupReturns(specs.User{
			UID:      2000,
			GID:      3000,
			Username: "vcap",
		}, nil)
	})

	It("convert a crucible config into a runc spec", func() {
		spec, err := specbuilder.Build(jobName, cfg, userIDFinder)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Version).To(Equal(specs.Version))

		Expect(spec.Platform).To(Equal(specs.Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		}))

		expectedProcessArgs := append([]string{cfg.Process.Executable}, cfg.Process.Args...)
		Expect(userIDFinder.LookupCallCount()).To(Equal(1))
		Expect(userIDFinder.LookupArgsForCall(0)).To(Equal("vcap"))
		Expect(spec.Process).To(Equal(&specs.Process{
			Terminal:    false,
			ConsoleSize: nil,
			User: specs.User{
				UID:      2000,
				GID:      3000,
				Username: "vcap",
			},
			Args: expectedProcessArgs,
			Env:  cfg.Process.Env,
			Cwd:  "/",
			Rlimits: []specs.LinuxRlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: uint64(1024),
					Soft: uint64(1024),
				},
			},
			NoNewPrivileges: true,
		}))

		Expect(spec.Root).To(Equal(specs.Root{
			Path: "/var/vcap/data/crucible/bundles/ambien-job/rootfs",
		}))

		Expect(spec.Hostname).To(Equal("ambien-job"))

		Expect(spec.Mounts).To(ConsistOf([]specs.Mount{
			{
				Destination: "/proc",
				Type:        "proc",
				Source:      "proc",
				Options:     nil,
			},
			{
				Destination: "/dev",
				Type:        "tmpfs",
				Source:      "tmpfs",
				Options:     []string{"nosuid", "noexec", "mode=755", "size=65536k"},
			},
			{
				Destination: "/dev/pts",
				Type:        "devpts",
				Source:      "devpts",
				Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
			},
			{
				Destination: "/dev/shm",
				Type:        "tmpfs",
				Source:      "shm",
				Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
			},
			{
				Destination: "/dev/mqueue",
				Type:        "mqueue",
				Source:      "mqueue",
				Options:     []string{"nosuid", "noexec", "nodev"},
			},
			{
				Destination: "/sys",
				Type:        "sysfs",
				Source:      "sysfs",
				Options:     []string{"nosuid", "noexec", "nodev", "ro"},
			},
			{
				Destination: "/sys/fs/cgroup",
				Type:        "cgroup",
				Source:      "cgroup",
				Options:     []string{"nosuid", "noexec", "nodev", "relatime", "ro"},
			},
			{
				Destination: "/bin",
				Type:        "bind",
				Source:      "/bin",
				Options:     []string{"nosuid", "nodev", "rbind", "ro"},
			},
			{
				Destination: "/etc",
				Type:        "bind",
				Source:      "/etc",
				Options:     []string{"nosuid", "nodev", "rbind", "ro"},
			},
			{
				Destination: "/usr",
				Type:        "bind",
				Source:      "/usr",
				Options:     []string{"nosuid", "nodev", "rbind", "ro"},
			},
			{
				Destination: "/lib",
				Type:        "bind",
				Source:      "/lib",
				Options:     []string{"nosuid", "nodev", "rbind", "ro"},
			},
			{
				Destination: "/lib64",
				Type:        "bind",
				Source:      "/lib64",
				Options:     []string{"nosuid", "nodev", "rbind", "ro"},
			},
			{
				Destination: "/var/vcap/jobs/ambien-job",
				Type:        "bind",
				Source:      "/var/vcap/jobs/ambien-job",
				Options:     []string{"rbind", "ro"},
			},
			{
				Destination: "/var/vcap/data/packages",
				Type:        "bind",
				Source:      "/var/vcap/data/packages",
				Options:     []string{"rbind", "ro"},
			},
			{
				Destination: "/var/vcap/packages",
				Type:        "bind",
				Source:      "/var/vcap/packages",
				Options:     []string{"rbind", "ro"},
			},
		}))

		Expect(spec.Linux.RootfsPropagation).To(Equal("private"))
		Expect(spec.Linux.MaskedPaths).To(ConsistOf([]string{
			"/proc/kcore",
			"/proc/latency_stats",
			"/proc/timer_list",
			"/proc/timer_stats",
			"/proc/sched_debug",
			"/sys/firmware",
		}))

		Expect(spec.Linux.ReadonlyPaths).To(ConsistOf([]string{
			"/proc/asound",
			"/proc/bus",
			"/proc/fs",
			"/proc/irq",
			"/proc/sys",
			"/proc/sysrq-trigger",
		}))

		Expect(spec.Linux.Namespaces).To(ConsistOf([]specs.LinuxNamespace{{Type: "uts"}, {Type: "mount"}}))
	})

	Context("when the user id lookup fails", func() {
		BeforeEach(func() {
			userIDFinder.LookupReturns(specs.User{}, errors.New("this user does not exist"))
		})

		It("returns an error", func() {
			_, err := specbuilder.Build(jobName, cfg, userIDFinder)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when there is no process specified", func() {
		BeforeEach(func() {
			cfg = &config.CrucibleConfig{}
		})

		It("returns an error", func() {
			_, err := specbuilder.Build(jobName, cfg, userIDFinder)
			Expect(err).To(MatchError("no process defined"))
		})
	})
})
