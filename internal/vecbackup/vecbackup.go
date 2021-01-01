package vecbackup

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	VC_VERSION              = 1
	VC_MAGIC                = "VBKC"
	VV_VERSION              = 1
	VV_MAGIC                = "VBKV"
	CONFIG_FILE             = "vecbackup-config"
	VERSION_DIR             = "versions"
	CHUNK_DIR               = "chunks"
	VERSION_FILENAME_PREFIX = "version-"
	LOCK_FILENAME           = "lock"
	RESTORE_TEMP_SUFFIX     = ".vbk.restore.temp"
	DEFAULT_DIR_PERM        = 0700
	DEFAULT_FILE_PERM       = 0600
	PATH_SEP                = string(os.PathSeparator)
)

var stdout io.Writer = os.Stdout
var stderr io.Writer = os.Stderr
var debug = false

func SetDebug(dbg bool) {
	debug = dbg
}

func debugP(fmt string, v ...interface{}) {
	if debug {
		log.Printf(fmt, v...)
	}
}

func readExcludeFile(fn string) ([]string, error) {
	if fn == "" {
		return nil, nil
	}
	excludePatterns := []string{}
	in, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		l := scanner.Text()
		if len(l) > 0 {
			_, err := filepath.Match(l, "zzz")
			if err != nil {
				return nil, fmt.Errorf("Bad exclude pattern: %s", l)
			} else {
				excludePatterns = append(excludePatterns, l)
			}
		}
	}
	return excludePatterns, nil
}

func toExclude(excludePatterns []string, dir, fn string) bool {
	for _, p := range excludePatterns {
		var matched bool
		var err error
		if p[0] == os.PathSeparator {
			matched, err = filepath.Match(p, filepath.ToSlash(PATH_SEP+path.Join(dir, fn)))
		} else {
			matched, err = filepath.Match(p, filepath.ToSlash(fn))
		}
		if err == nil && matched {
			return true
		}
	}
	return false
}

func isSymlink(f os.FileInfo) bool {
	return f.Mode()&os.ModeSymlink != 0
}

func pathCompare(p1, p2 string) int {
	l1 := strings.Split(p1, PATH_SEP)
	n1 := len(l1)
	l2 := strings.Split(p2, PATH_SEP)
	n2 := len(l2)
	for i := 0; ; i++ {
		if i < n1 && i < n2 {
			if r := strings.Compare(l1[i], l2[i]); r != 0 {
				return r
			}
		} else if i == n1 && i == n2 {
			return 0
		} else if n1 < n2 {
			return -1
		} else {
			return 1
		}
	}
}

type byDirFile []string

func (s byDirFile) Len() int {
	return len(s)
}
func (s byDirFile) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byDirFile) Less(i, j int) bool {
	return pathCompare(s[i], s[j]) < 0
}

func nodeExists(p string) bool {
	_, err := os.Lstat(p)
	return !os.IsNotExist(err)
}

type fileDataMap struct {
	names []string
	files map[string]*FileData
}

func (fdm *fileDataMap) Init() {
	fdm.files = make(map[string]*FileData)
	fdm.names = nil
}

func (fdm *fileDataMap) AddItem(fd *FileData) bool {
	n := fd.Name
	if fdm.files[n] == nil {
		fdm.files[n] = fd
		fdm.names = append(fdm.names, n)
		return true
	}
	return false
}

func scanOneDir(src string, f os.FileInfo, excludes []string, fdm *fileDataMap) int {
	if f == nil {
		if fi, err := os.Lstat(src); err != nil {
			fmt.Fprintf(stderr, "F %s: %s\n", src, err)
			return 1
		} else {
			f = fi
		}
	}
	if f.IsDir() {
		if fdm.AddItem(NewDirectory(src, f.Mode().Perm())) {
			if files, err := ioutil.ReadDir(src); err != nil {
				fmt.Fprintf(stderr, "F %s: %s\n", src, err)
				return 1
			} else {
				errs := 0
				for _, child := range files {
					if toExclude(excludes, src, child.Name()) {
						continue
					}
					errs2 := scanOneDir(path.Join(src, child.Name()), child, excludes, fdm)
					errs = errs + errs2
				}
				return errs
			}
		}
	} else if f.Mode().IsRegular() {
		fdm.AddItem(NewRegularFile(src, f.Size(), f.ModTime(), f.Mode().Perm(), nil, nil, nil))
	} else if isSymlink(f) {
		if target, err := os.Readlink(src); err != nil {
			fmt.Fprintf(stderr, "F %s: %s\n", src, err)
			return 1
		} else {
			fdm.AddItem(NewSymlink(src, target))
		}
	}
	return 0
}

func scanSrcs(excludes []string, srcs []string) (*fileDataMap, int) {
	errs := 0
	fdm := &fileDataMap{}
	fdm.Init()
	for _, src := range srcs {
		src = filepath.Clean(src)
		err2 := scanOneDir(src, nil, excludes, fdm)
		errs = errs + err2
	}
	return fdm, errs
}

type sharedBuf struct {
	bf []byte
}

func (b *sharedBuf) Realloc(size int) {
	if cap(b.bf) < size {
		b.bf = make([]byte, size)
	}
	b.bf = b.bf[:size]
}

func (b *sharedBuf) B() []byte {
	return b.bf
}

func makeChunkFP(secret []byte, origFp FP) FP {
	if secret == nil {
		return origFp
	}
	s := append(append([]byte(nil), secret...), origFp[:]...)
	return sha512.Sum512_256(s)
}

func matchChunkFP(secret []byte, fp FP, b []byte) bool {
	fp2 := makeChunkFP(secret, sha512.Sum512_256(b))
	return fp == fp2
}

func statsRemoveFile(fd *FileData, stats *BackupStats) {
	if fd != nil {
		if fd.IsFile() {
			stats.FilesRemoved++
		} else if fd.IsDir() {
			stats.DirsRemoved++
		} else if fd.IsSymlink() {
			stats.SymlinksRemoved++
		}
	}
}

func backupOneNode(cm *CMgr, cs int32, dryRun, force, verbose bool, buf *sharedBuf, out io.Writer, old *FileData, new *FileData, secret []byte, stats *BackupStats) (*FileData, error) {
	if old != nil && new == nil {
		statsRemoveFile(old, stats)
		if verbose {
			fmt.Fprintf(out, "- %s\n", old.PrettyPrint())
		}
		return nil, nil
	}
	to_add := old
	if force || old == nil && new != nil || new.Type != old.Type || (new.IsFile() && (new.Size != old.Size || !new.ModTime.Equal(old.ModTime))) || (new.IsSymlink() && new.Target != old.Target) {
		to_add = new
	} else if old.IsFile() {
		for _, chunk := range old.Chunks {
			if !cm.FindChunk(chunk) {
				fmt.Fprintf(stderr, "Missing chunk %s\n", chunk)
				to_add = new
				break
			}
		}
	}
	var addSrcSize int64 = 0
	var addRepoSize int64 = 0
	if to_add == new {
		if new.IsFile() {
			if !dryRun {
				if added, added2, err := addChunks(new, cm, cs, dryRun, buf, secret); err != nil {
					fmt.Fprintf(stderr, "F %s: %s\n", to_add.PrettyPrint(), err)
					stats.Errors++
					return nil, err
				} else {
					addSrcSize = added
					addRepoSize = added2
				}
			}
			if old == nil || !old.IsFile() {
				stats.FilesNew++
				statsRemoveFile(old, stats)
			} else {
				stats.FilesUpdated++
			}
		} else if new.IsDir() {
			if old == nil || !old.IsDir() {
				stats.DirsNew++
				statsRemoveFile(old, stats)
			} else {
				stats.DirsUpdated++
			}
		} else if new.IsSymlink() {
			if old == nil || !old.IsSymlink() {
				stats.SymlinksNew++
				statsRemoveFile(old, stats)
			} else {
				stats.SymlinksUpdated++
			}
		}
		if verbose {
			fmt.Fprintf(out, "+ %v\n", to_add.PrettyPrint())
		}
	} else {
		if !old.IsSymlink() {
			old.Perm = new.Perm
		}
	}
	if to_add.IsFile() {
		stats.Files++
		stats.Size += new.Size
		stats.AddSrcSize += addSrcSize
		stats.AddRepoSize += addRepoSize
	} else if to_add.IsDir() {
		stats.Dirs++
	} else {
		stats.Symlinks++
	}
	return to_add, nil
}

func addChunks(fd *FileData, cm *CMgr, bs int32, dryRun bool, buf *sharedBuf, secret []byte) (int64, int64, error) {
	h := sha512.New512_256()
	var chunks []FP = nil
	var sizes []int32
	file, err := os.Open(fd.Name)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	buf.Realloc(int(bs))
	var n int64 = 0
	var added int64 = 0
	var addedActual int64 = 0
	for {
		count, err := io.ReadFull(file, buf.B())
		if count > 0 {
			b := buf.B()[:count]
			n += int64(count)
			h.Write(b)
			var chunk FP = makeChunkFP(secret, sha512.Sum512_256(b))
			dup, compressedLen, err := cm.AddChunk(chunk, b)
			if err != nil {
				return 0, 0, err
			}
			if !dup {
				added = added + int64(count)
				addedActual = addedActual + int64(compressedLen)
			}
			chunks = append(chunks, chunk)
			sizes = append(sizes, int32(count))
		}
		if n > fd.Size {
			return 0, 0, fmt.Errorf("File size changed %s", fd.Name)
		}
		if err == io.EOF || count < int(bs) {
			if n < fd.Size {
				return 0, 0, fmt.Errorf("File size changed %s", fd.Name)
			}
			break
		}
	}
	fd.Chunks = chunks
	fd.Sizes = sizes
	fd.FileChecksum = h.Sum(nil)
	return added, addedActual, nil
}

func matchRestorePatterns(fp string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pat := range patterns {
		if matchRestorePattern(fp, pat) {
			return true
		}
	}
	return false
}

func matchRestorePattern(p, pat string) bool {
	if p == pat {
		return true
	}
	l := len(pat)
	return l == 0 || (len(p) > l && p[:l] == pat && os.IsPathSeparator(p[l]))
}

func restoreFileToTemp(fd *FileData, cm *CMgr, fn string, testRun bool, secret []byte) error {
	var f *os.File
	if !testRun {
		d := filepath.Dir(fn)
		err := os.MkdirAll(d, DEFAULT_DIR_PERM)
		f, err = os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, DEFAULT_FILE_PERM)
		if err != nil {
			return err
		}
		defer f.Close()
	}
	var l int64
	h := sha512.New512_256()
	for _, chunk := range fd.Chunks {
		b, err := cm.ReadChunk(chunk)
		if err == nil && !matchChunkFP(secret, chunk, b) {
			err = fmt.Errorf("Bad chunk, mismatch fingerpint %s", chunk)
		}
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Missing chunk %s", chunk)
			} else {
				return fmt.Errorf("Bad chunk %s", chunk)
			}
		}
		h.Write(b)
		l += int64(len(b))
		if !testRun {
			_, err = f.Write(b)
			if err != nil {
				return err
			}
		}
	}
	if l != fd.Size {
		return fmt.Errorf("Length mismatch: %d vs %d", l, fd.Size)
	}
	if bytes.Compare(fd.FileChecksum, h.Sum(nil)) != 0 {
		return errors.New("File checksum mismatch")
	}
	return nil
}

func restoreNode(fd *FileData, cm *CMgr, recDir string, testRun bool, merge bool, secret []byte) error {
	p := path.Join(recDir, fd.Name)
	if fd.IsDir() {
		if !testRun {
			if err := os.MkdirAll(p, DEFAULT_DIR_PERM); err != nil && !os.IsExist(err) {
				return err
			}
			return os.Chmod(p, fd.Perm|0700)
		}
		return nil
	}
	if fd.IsSymlink() {
		if merge {
			fi, err := os.Lstat(p)
			if !os.IsNotExist(err) {
				if err != nil {
					return err
				}
				if !isSymlink(fi) {
					return errors.New("Cannot restore symlink. File/dir already exist at the path.")
				}
				target, err := os.Readlink(p)
				if err != nil {
					return err
				}
				if target != fd.Target {
					return errors.New("Cannot restore symlink. Existing symlink points to wrong target.")
				}
				return nil
			}
		}
		if !testRun {
			return os.Symlink(fd.Target, p)
		}
		return nil
	}
	if merge {
		if fi, err := os.Lstat(p); err == nil && fi.Size() == fd.Size && fi.ModTime().Equal(fd.ModTime) {
			return nil
		}
	}
	tp := p + RESTORE_TEMP_SUFFIX
	err := restoreFileToTemp(fd, cm, tp, testRun, secret)
	if err == nil {
		if testRun {
			return nil
		}
		err = os.Chtimes(tp, fd.ModTime, fd.ModTime)
		if err == nil {
			err = os.Chmod(tp, fd.Perm)
			if err == nil {
				err = os.Rename(tp, p)
				if err == nil {
					return nil
				}
			}
		}
	}
	if tp != "" {
		os.Remove(tp)
	}
	return err
}

func setup(repo string, pwFile string) (*VMgr, *CMgr, *Config, error) {
	sm, repo2 := GetStorageMgr(repo)
	cfg, err := GetConfig(pwFile, sm, repo2)
	if err != nil {
		return nil, nil, nil, err
	}
	vm := MakeVMgr(sm, repo2, cfg.EncryptionKey)
	cm, err := MakeCMgr(sm, repo2, cfg.EncryptionKey, cfg.Compress)
	if err != nil {
		return nil, nil, nil, err
	}
	return vm, cm, cfg, nil
}

//---------------------------------------------------------------------------

func InitRepo(pwFile, repo string, chunkSize int32, iterations int, compress CompressionMode) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	sm, repo2 := GetStorageMgr(repo)
	files, err := sm.LsDir(repo2)
	if !os.IsNotExist(err) && len(files) != 0 {
		return fmt.Errorf("Backup repository already exists: %s", repo)
	}
	err = sm.MkdirAll(repo2)
	if err != nil {
		return fmt.Errorf("Cannot create repo dir: %s", err)
	}
	cfg := &Config{ChunkSize: chunkSize, Compress: compress}
	err = WriteNewConfig(pwFile, sm, repo2, iterations, cfg)
	if err != nil {
		return fmt.Errorf("Cannot write encypted config file: %s", err)
	}
	return err
}

type BackupStats struct {
	Version         string
	Dirs            int
	DirsNew         int
	DirsUpdated     int
	DirsRemoved     int
	Files           int
	FilesUpdated    int
	FilesNew        int
	FilesRemoved    int
	Symlinks        int
	SymlinksNew     int
	SymlinksUpdated int
	SymlinksRemoved int
	Errors          int
	Size            int64
	AddSrcSize      int64
	AddRepoSize     int64
}

func Backup(pwFile, repo, excludeFrom, setVersion string, dryRun, force, verbose bool, lockFile string, srcs []string, stats *BackupStats) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	if len(srcs) == 0 {
		return errors.New("At least one backup src must be specified")
	}
	var sml StorageMgr
	var lockFile2 string
	if lockFile == "" {
		var repo2 string
		sml, repo2 = GetStorageMgr(repo)
		lockFile = sml.JoinPath(repo, LOCK_FILENAME)
		lockFile2 = sml.JoinPath(repo2, LOCK_FILENAME)
	} else {
		sml, lockFile2 = GetStorageMgr(lockFile)
	}
	if err := sml.WriteLockFile(lockFile2); os.IsExist(err) {
		return fmt.Errorf("Repository is locked. Lock file %s exists.", lockFile)
	} else if err != nil {
		return err
	}
	defer sml.RemoveLockFile(lockFile2)
	vm, cm, cfg, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	excludePatterns, err := readExcludeFile(excludeFrom)
	if err != nil {
		return fmt.Errorf("Cannot read exclude-from file: %s", err)
	}
	var new_version string
	if setVersion != "" {
		if _, ok := DecodeVersionTime(setVersion); ok {
			new_version = setVersion
		} else {
			return fmt.Errorf("Invalid version %s", setVersion)
		}
	}
	last_version, err := vm.GetLatestVersion()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	if new_version == "" {
		new_version = CreateNewVersion(last_version)
	}
	sfdm, errs := scanSrcs(excludePatterns, srcs)
	stats.Errors += errs
	if len(sfdm.names) == 0 {
		return errors.New("Nothing to back up.")
	}
	var vfdm = &fileDataMap{}
	vfdm.Init()
	if last_version != "" {
		fds, err, errs := vm.LoadFiles(last_version)
		if err != nil {
			return fmt.Errorf("Failed reading previous version: %s", err)
		}
		stats.Errors += errs
		for _, fd := range fds {
			if !vfdm.AddItem(fd) {
				fmt.Fprintf(stderr, "Ignoring duplicate item in version file: %s.\n", fd.Name)
			}
		}
	}
	buf := &sharedBuf{}
	comb := append(append([]string(nil), vfdm.names...), sfdm.names...)
	sort.Strings(comb)
	sort.Sort(byDirFile(comb))
	var last string
	var fds []*FileData
	for _, n := range comb {
		if n == last {
			continue
		}
		last = n
		new_fd, err := backupOneNode(cm, cfg.ChunkSize, dryRun, force, verbose, buf, stdout, vfdm.files[n], sfdm.files[n], cfg.FPSecret, stats)
		if err == nil && new_fd != nil {
			fds = append(fds, new_fd)
		}
	}
	if !dryRun {
		if err = vm.SaveFiles(new_version, fds); err != nil {
			return err
		}
		stats.Version = new_version
	}
	return nil
}

func Restore(pwFile, repo, recDir, version string, merge, testRun, dryRun, verbose bool, patterns []string) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	if recDir == "" && !testRun && !dryRun {
		return errors.New("Target must be specified.")
	}
	vm, cm, cfg, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	if nodeExists(recDir) && !merge {
		return fmt.Errorf("Restore dir %s already exists", recDir)
	}
	if version == "" {
		version, err = vm.GetLatestVersion()
		if err != nil {
			return fmt.Errorf("Cannot read version files: %s", err)
		}
		if version == "" {
			return errors.New("Error! Nothing to restore. Repository is empty")
		}
	}
	fds, err, errs := vm.LoadFiles(version)
	if err != nil {
		return fmt.Errorf("Cannot read version file: %s", err)
	}
	if !testRun && !dryRun {
		os.MkdirAll(recDir, DEFAULT_DIR_PERM)
	}
	for _, fd := range fds {
		if matchRestorePatterns(fd.Name, patterns) {
			if !dryRun {
				err = restoreNode(fd, cm, recDir, testRun, merge, cfg.FPSecret)
			}
			if err == nil {
				if verbose {
					fmt.Fprintf(stdout, "%s\n", fd.PrettyPrint())
				}
			} else {
				fmt.Fprintf(stderr, "F %s: %s\n", fd.PrettyPrint(), err)
				errs++
			}
		}
	}
	for i := len(fds) - 1; i >= 0; i-- {
		fd := fds[i]
		if matchRestorePatterns(fd.Name, patterns) {
			if !dryRun && !testRun && fd.IsDir() {
				err = os.Chmod(path.Join(recDir, fd.Name), fd.Perm)
				if err != nil {
					fmt.Fprintf(stderr, "F %s: %s\n", fd.PrettyPrint(), err)
					errs++
				}
			}
		}
	}
	if errs > 0 {
		return errors.New("Errors occured during restore. Some files were not restored.")
	}
	return nil
}

func Ls(pwFile, repo, version string) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, _, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	if version == "" {
		version, err = vm.GetLatestVersion()
	}
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	} else if version == "" {
		return nil
	}
	fds, err, errs := vm.LoadFiles(version)
	if err != nil {
		return fmt.Errorf("Cannot read version file: %s", err)
	}
	for _, fd := range fds {
		fmt.Fprintf(stdout, "%s\n", fd.PrettyPrint())
	}
	if errs > 0 {
		return errors.New("Error! Some file info were invalid.")
	}
	return nil
}

func Versions(pwFile, repo string) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, _, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	for _, v := range versions {
		fmt.Fprintf(stdout, "%s\n", v)
	}
	return nil
}

func DeleteVersion(pwFile, repo, version string) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	if version == "" {
		return errors.New("Version must be specified.")
	}
	vm, _, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	err = vm.DeleteVersion(version)
	if err != nil {
		return fmt.Errorf("Cannot delete version %s: %s", version, err)
	}
	return nil
}

func DeleteOldVersions(pwFile, repo string, dryRun bool) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, _, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	d := ReduceVersions(time.Now(), versions)
	for _, v := range d {
		fmt.Fprintf(stdout, "Deleting version %s\n", v)
		if !dryRun {
			err = vm.DeleteVersion(v)
			if err != nil {
				return fmt.Errorf("Cannot delete version %s: %s", v, err)
			}
		}
	}
	return nil
}

type VerifyRepoResults struct {
	Chunks, Ok, Errors, Missing, Unused int
}

func VerifyRepo(pwFile, repo string, quick bool, r *VerifyRepoResults) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, cm, cfg, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	counts := cm.GetAllChunks()
	fail := false
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))
	for _, v := range versions {
		var chunksError, chunksMissing int
		var size int64
		fds, err, nerrs := vm.LoadFiles(v)
		if err != nil {
			fmt.Fprintf(stderr, "Failed to read version %s : %s\n", v, err)
			fail = true
			continue
		}
		vChunks := make(map[FP]int)
		for _, fd := range fds {
			for i, chunk := range fd.Chunks {
				size = size + int64(fd.Sizes[i])
				vChunks[chunk]++
				if vChunks[chunk] > 1 {
					continue
				}
				counts[chunk]++
				if counts[chunk] == 1 {
					if quick {
						if cm.FindChunk(chunk) {
							r.Ok++
						} else {
							r.Missing++
							chunksMissing++
						}
					} else {
						b, err := cm.ReadChunk(chunk)
						if err == nil && !matchChunkFP(cfg.FPSecret, chunk, b) {
							err = fmt.Errorf("Mismatch fingerpint %s", chunk)
						}
						if err != nil {
							if os.IsNotExist(err) {
								fmt.Fprintf(stderr, "Missing chunk %s\n", chunk)
								r.Missing++
								chunksMissing++
							} else {
								fmt.Fprintf(stderr, "Error chunk %s : %s\n", chunk, err)
								r.Errors++
								chunksError++
							}
						} else {
							r.Ok++
						}
					}
				}
			}
		}
		chunks := len(vChunks)
		chunksOk := chunks - chunksError - chunksMissing
		if quick {
			if nerrs > 0 {
				fmt.Fprintf(stdout, "Version %s : %d bytes, %d chunk(s), %d missing. %d invalid file info.\n", v, size, chunks, chunksMissing, nerrs)
			} else {
				fmt.Fprintf(stdout, "Version %s : %d bytes, %d chunk(s), %d missing.\n", v, size, chunks, chunksMissing)
			}
		} else {
			if nerrs > 0 {
				fmt.Fprintf(stdout, "Version %s : %d bytes, %d chunk(s), %d good, %d bad, %d missing. %d invalid file info.\n", v, size, chunks, chunksOk, chunksError, chunksMissing, nerrs)
			} else {
				fmt.Fprintf(stdout, "Version %s : %d bytes, %d chunk(s), %d good, %d bad, %d missing.\n", v, size, chunks, chunksOk, chunksError, chunksMissing)
			}
		}
	}
	for _, count := range counts {
		r.Chunks++
		if count == 0 {
			r.Unused++
		}
	}
	fmt.Fprintf(stdout, "Summary: %d chunk(s), %d good, %d bad, %d missing, %d unused\n", r.Chunks, r.Ok, r.Errors, r.Missing, r.Unused)
	if fail || r.Errors > 0 || r.Missing > 0 {
		return errors.New("Error verifying repo.")
	}
	return nil
}

func PurgeUnused(pwFile, repo string, dryRun, verbose bool) error {
	if repo == "" {
		return errors.New("Backup repository must be specified.")
	}
	vm, cm, _, err := setup(repo, pwFile)
	if err != nil {
		return err
	}
	versions, err := vm.GetVersions()
	if err != nil {
		return fmt.Errorf("Cannot read version files: %s", err)
	}
	counts := cm.GetAllChunks()
	total_chunks := len(counts)
	for _, v := range versions {
		fds, err, errs := vm.LoadFiles(v)
		if err != nil {
			return fmt.Errorf("Cannot read version file: %s", err)
		}
		if errs > 0 {
			return fmt.Errorf("Error! Some file info were invalid in version %s", v)
		}
		for _, fd := range fds {
			for _, chunk := range fd.Chunks {
				delete(counts, chunk)
			}
		}
	}
	numDeleted := 0
	numFailed := 0
	for chunk, _ := range counts {
		if dryRun {
			if verbose {
				fmt.Fprintf(stdout, "Delete %s\n", chunk)
			}
			numDeleted++
		} else {
			if err := cm.DeleteChunk(chunk); err != nil {
				numFailed++
				if verbose {
					fmt.Fprintf(stdout, "Failed to delete %s: %s\n", chunk, err)
				}
			} else {
				numDeleted++
				if verbose {
					fmt.Fprintf(stdout, "Deleted %s\n", chunk)
				}
			}
		}
	}
	if dryRun {
		fmt.Fprintf(stdout, "Chunks to be purged (dryrun): %d out of %d.\n", numDeleted, total_chunks)
	} else {
		fmt.Fprintf(stdout, "Chunks purged: %d out of %d.\n", numDeleted, total_chunks)
	}
	if numFailed > 0 {
		return fmt.Errorf("Failed to purge %d chunk(s).", numFailed)
	}
	return nil
}

func RemoveLock(repo, lockFile string) error {
	var sml StorageMgr
	var lockFile2 string
	if lockFile == "" {
		var repo2 string
		sml, repo2 = GetStorageMgr(repo)
		lockFile2 = sml.JoinPath(repo2, LOCK_FILENAME)
	} else {
		sml, lockFile2 = GetStorageMgr(lockFile)
	}
	return sml.RemoveLockFile(lockFile2)
}
