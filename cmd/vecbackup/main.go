package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/ptsim/vecbackup/internal/vecbackup"
	"math"
	"os"
	"runtime/pprof"
)

func usageAndExit() {
	fmt.Fprintf(os.Stderr, `Usage:
  vecbackup help
  vecbackup init [-pw <pwfile>] [-chunk-size size] [-pbkdf2-iterations num] -r <repo>
  vecbackup backup [-v] [-f] [-n] [-version <version>] [-pw <pwfile>] [-exclude-from <file>] [-lock-file <file>] [-check-chunks] [-max-dop n] -r <repo> <src> [<src> ...]
  vecbackup ls [-version <version>] [-pw <pwfile>] -r <repo>
  vecbackup versions [-pw <pwfile>] -r <repo>
  vecbackup restore [-v] [-n] [-version <version>] [-merge] [-pw <pwfile>] [-verify-only] [-max-dop n] -r <repo> -target <restoredir> [<path> ...]
  vecbackup delete-version [-pw <pwfile>] -r <repo> -version <version>
  vecbackup delete-old-versions [-n] [-pw <pwfile>] -r <repo>
  vecbackup verify-repo [-pw <pwfile>] [-quick] [-max-dop n] -r <repo>
  vecbackup purge-unused [-v] [-pw <pwfile>] [-n] -r <repo>
  vecbackup remove-lock [-r <repo>] [-lock-file <file>]
`)
	os.Exit(1)
}

func help() {
	fmt.Printf(`Usage:
  vecbackup help
  vecbackup init [-pw <pwfile>] [-chunk-size size] [-pbkdf2-iterations num] [-compress mode] -r <repo>
      -chunk-size   files are broken into chunks of this size.
      -pbkdf2-iterations
                    number of iterations for PBKDF2 key generation.
                    Minimum 100,000.
      -compress     Compress mode. Default auto. Modes:
                      auto     Compresses most chunks but skip small chunks
                               and only check if compression saves space on
                               a small prefix of large chunks.
                      slow     Tries to every chunk. Keeps the uncompressed
                               version if it is smaller.
                      no       Never compress chunks.
                      yes      Compress all chunks.

    Initialize a new backup repository.

  vecbackup backup [-v] [-f] [-n] [-version <version>] [-pw <pwfile>] [-exclude-from <file>] [-lock-file <file>] [-check-chunks] [-max-dop n] -r <repo> <src> [<src> ...]
    Incrementally and recursively backs up one or more <src> to <repo>.
    The files, directories and symbolic links backed up. Other file types are silently ignored.
    Files that have not changed in same size and timestamp are not backed up.
    A lock file is created to prevent starting another backup operation when one is
    already in progress. It is removed when done. Running simultaneous backups isn't
    recommended. It is slow because the second backup is repeating the work of the first.
      -v            verbose, prints the items that are added (+) or removed (-).
      -f            force, always check file contents 
      -check-chunks check and add missing chunks
      -n            dry run, shows what would have been backed up.
      -version      save as the given version, instead of the current time
      -exclude-from reads list of exclude patterns from specified file
      -lock-file    path to lock file if different from default (<repo>/lock)

  vecbackup versions [-pw <pwfile>] -r <repo>
    Lists all backup versions in chronological order. The version name is a
    timestamp in UTC formatted with RFC3339Nano format (YYYY-MM-DDThh:mm:ssZ).

  vecbackup ls [-version <version>] [-pw <pwfile>] -r <repo>
    Lists files in <repo>.
    -version <version>   list the files in that version

  vecbackup restore [-v] [-n] [-version <version>] [-merge] [-pw <pwfile>] [-verify-only] [-max-dop n] -r <repo> -target <restoredir> [<path> ...]
    Restores all the items or the given <path>s to <restoredir>.
      -v            verbose, prints the names of all items restored
      -n            dry run, shows what would have been restored.
      -version <version>
                    restore that given version or that latest version if not specified.
      -merge        merge the restored files into the given target
                    if it already exists. Files of the same size and timestamp
                    are not extracted again. This can be used to resume
                    a previous restore operation.
      -verify-only  reconstruct the files by the reading chunks and verifying the checksums but do not 
                    write them out. -target is not needed. 
      -target <restoredir>
                    target dir for the restore. It must not already exist unless -merge is specified.
                    The target dir must specified except if -verify-only is specified.

  vecbackup delete-version [-pw <pwfile>] -r <repo> -verson <version>
    Deletes the given version. No chunks are deleted.

  vecbackup delete-old-versions [-n] [-pw <pwfile>] -r <repo>
    Deletes old versions. No chunks are deleted.
    Keeps all versions within one day, one version per hour for the last week,
    one version per day in the last month, one version per week in the last 
    year and one version per month otherwise.
      -n            dry run, shows versions that would have been deleted

  vecbackup verify-repo [-pw <pwfile>] [-quick] [-max-dop n] -r <repo>
    Verifies that all the chunks used by all the files in all versions
    can be read and match their checksums.
      -quick        Quick, just checks that the chunks exist.

  vecbackup purge-unused [-pw <pwfile>] [-n] -r <repo>
    Deletes chunks that are not used by any file in any backup version.
      -n            dry run, shows number of chunks to be deleted.
      -v            prints the chunks being deleted

  vecbackup remove-lock [-lock-file <file>] [-r repo]
      -lock-file    path to lock file if different from default (<repo>/lock)
    Removes the lock file left behind due to a failed backup operation.
    Either -r or -lock-file must be specified.

Common flags:
      -r            Path to backup repository.
      -pw           file containing the password
      -rclone-binary  Path to the "rclone" program
      -max-dop      maximum degree of parallelism. Default 3. 
                    Minimum 1. Maximum 100. Increasing this number increases
                    memory, cpu, disk and network usage but reduces total time.
                    Only used for backup, restore and verify-repo commands.

Remote repository:
  If the repository path starts with "rclone:", the rest of the path is passed to rclone
  as the location of the repository. For example, if the repo path is "rclone:remote:path/to/dir",
  the rclone path used to store the repo is "remote:path/to/dir".
  If the repository path does not start with "rclone:", it is assumed to be a local path.

Exclude Patterns:

  Patterns that do not start with a '/' are matched against the filename only.
  Patterns that start with a '/' are matched against the sub-path relative
  to src directory.
  * matches any sequence of non-separator characters.
  ? matches any single non-separator character.
  See https://golang.org/pkg/path/filepath/#Match
`)
}

var debugF = flag.Bool("debug", false, "Show debug info.")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var memprofile = flag.String("memprofile", "", "write memory profile to file")

var verbose = flag.Bool("v", false, "Verbose")
var force = flag.Bool("f", false, "Force. Always check file contents.")
var checkChunks = flag.Bool("check-chunks", false, "Check and add missing chunks.")
var dryRun = flag.Bool("n", false, "Dry run.")
var verifyOnly = flag.Bool("verify-only", false, "Verify but don't write.")
var version = flag.String("version", "", "The version to operate on.")
var merge = flag.Bool("merge", false, "Merge into existing directory.")
var pwFile = flag.String("pw", "", "File containing password.")
var chunkSize = flag.Int("chunk-size", 16*1024*1024, "Chunk size.")
var iterations = flag.Int("pbkdf2-iterations", 100000, "PBKDF2 iteration count.")
var repo = flag.String("r", "", "Path to backup repository.")
var target = flag.String("target", "", "Path to restore target path.")
var excludeFrom = flag.String("exclude-from", "", "Reads list of exclude patterns from specified file.")
var compress = flag.String("compress", "auto", "Compression mode")
var quick = flag.Bool("quick", false, "Quick mode")
var rclone = flag.String("rclone-binary", "rclone", "Path to rclone binary")
var lockFile = flag.String("lock-file", "", "Lock file path")
var maxDop = flag.Int("max-dop", 3, "Maximum degree of parallelism.")

func exitIfError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) < 2 {
		usageAndExit()
	}
	cmd := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	flag.Parse()
	vecbackup.SetDebug(*debugF)
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(fmt.Sprintf("could not create cpu profile: %v", err))
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		defer func() {
			f, err := os.Create(*memprofile)
			if err != nil {
				panic(fmt.Sprintf("could not create memory profile: %v", err))
			}
			//runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				panic(fmt.Sprintf("could not write memory profile: %v", err))
			}
			f.Close()
		}()
	}
	vecbackup.SetRcloneBinary(*rclone)
	if cmd == "help" {
		help()
	} else if cmd == "backup" {
		var stats vecbackup.BackupStats
		if *maxDop < 1 || *maxDop > 100 {
			exitIfError(errors.New("-max-dop must be between 1 and 100.\n"))
		}
		exitIfError(vecbackup.Backup(*pwFile, *repo, *excludeFrom, *version, *dryRun, *force, *checkChunks, *verbose, *lockFile, *maxDop, flag.Args(), &stats))
		if *dryRun {
			fmt.Printf("Backup dry run\n%d dir(s) (%d new %d updated %d removed)\n%d file(s) (%d new %d updated %d removed)\n%d symlink(s) (%d new %d updated %d removed)\ntotal src size %d\n%d error(s).\n", stats.Dirs, stats.DirsNew, stats.DirsUpdated, stats.DirsRemoved, stats.Files, stats.FilesNew, stats.FilesUpdated, stats.FilesRemoved, stats.Symlinks, stats.SymlinksNew, stats.SymlinksUpdated, stats.SymlinksRemoved, stats.Size, stats.Errors)
		} else {
			newRepoPct := float64(100.0)
			if stats.SrcAdded > 0 {
				newRepoPct = float64(stats.RepoAdded) * 100 / float64(stats.SrcAdded)
			}
			fmt.Printf("Backup version %s\n%d dir(s) (%d new %d updated %d removed)\n%d file(s) (%d new %d updated %d removed)\n%d symlink(s) (%d new %d updated %d removed)\ntotal src size %d, new src size %d, repo added %d (%0.1f%% of new src size)\n%d error(s).\n", stats.Version, stats.Dirs, stats.DirsNew, stats.DirsUpdated, stats.DirsRemoved, stats.Files, stats.FilesNew, stats.FilesUpdated, stats.FilesRemoved, stats.Symlinks, stats.SymlinksNew, stats.SymlinksUpdated, stats.SymlinksRemoved, stats.Size, stats.SrcAdded, stats.RepoAdded, newRepoPct, stats.Errors)
		}
		if stats.Errors > 0 {
			exitIfError(errors.New(fmt.Sprintf("%d errors encountered. Some data were not backed up.", stats.Errors)))
		}
	} else if cmd == "restore" {
		if *maxDop < 1 || *maxDop > 100 {
			exitIfError(errors.New("-max-dop must be between 1 and 100.\n"))
		}
		exitIfError(vecbackup.Restore(*pwFile, *repo, *target, *version, *merge, *verifyOnly, *dryRun, *verbose, *maxDop, flag.Args()))
	} else if flag.NArg() > 0 {
		usageAndExit()
	} else if cmd == "init" {
		if *chunkSize > math.MaxInt32 {
			exitIfError(errors.New("Chunk size is too big."))
		}
		if *iterations < 100000 {
			exitIfError(errors.New(fmt.Sprintf("Too few PBKDF2 iterations, minimum 100,000: %d", *iterations)))
		}
		var mode vecbackup.CompressionMode = vecbackup.CompressionMode_AUTO
		if *compress == "auto" {
			mode = vecbackup.CompressionMode_AUTO
		} else if *compress == "slow" {
			mode = vecbackup.CompressionMode_SLOW
		} else if *compress == "yes" {
			mode = vecbackup.CompressionMode_YES
		} else if *compress == "no" {
			mode = vecbackup.CompressionMode_NO
		} else {
			exitIfError(errors.New("Invalid -compress flag."))
		}
		exitIfError(vecbackup.InitRepo(*pwFile, *repo, int32(*chunkSize), *iterations, mode))
	} else if cmd == "ls" {
		exitIfError(vecbackup.Ls(*pwFile, *repo, *version))
	} else if cmd == "versions" {
		exitIfError(vecbackup.Versions(*pwFile, *repo))
	} else if cmd == "delete-version" {
		exitIfError(vecbackup.DeleteVersion(*pwFile, *repo, *version))
	} else if cmd == "delete-old-versions" {
		exitIfError(vecbackup.DeleteOldVersions(*pwFile, *repo, *dryRun))
	} else if cmd == "verify-repo" {
		var r vecbackup.VerifyRepoResults
		if *maxDop < 1 || *maxDop > 100 {
			exitIfError(errors.New("-max-dop must be between 1 and 100.\n"))
		}
		exitIfError(vecbackup.VerifyRepo(*pwFile, *repo, *quick, *maxDop, &r))
	} else if cmd == "purge-unused" {
		exitIfError(vecbackup.PurgeUnused(*pwFile, *repo, *dryRun, *verbose))
	} else if cmd == "remove-lock" {
		if *repo == "" && *lockFile == "" {
			exitIfError(errors.New("Either -r or -lock-file must be specified."))
		}
		exitIfError(vecbackup.RemoveLock(*repo, *lockFile))
	} else {
		usageAndExit()
	}
}
