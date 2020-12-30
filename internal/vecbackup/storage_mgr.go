package vecbackup

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

var rcloneBinary string = "rclone"

func SetRcloneBinary(p string) {
	rcloneBinary = p
}

type StorageMgrLsDir2Func func(dir, file string)

type StorageMgr interface {
	JoinPath(d, f string) string
	LsDir(p string) ([]string, error)
	LsDir2(p string, f StorageMgrLsDir2Func) error
	ExistInDir(d, f string) (bool, error)
	MkdirAll(p string) error
	ReadFile(p string) ([]byte, error)
	WriteFile(p string, d []byte) error
	DeleteFile(p string) error
	WriteLockFile(p string) error
	RemoveLockFile(p string) error
}

type rcloneSMgr struct{}
type localSMgr struct{}

var TheRcloneSMgr = rcloneSMgr{}
var TheLocalSMgr = localSMgr{}

func GetStorageMgr(p string) (StorageMgr, string) {
	if len(p) > 7 && p[:7] == "rclone:" {
		return TheRcloneSMgr, p[7:]
	}
	return TheLocalSMgr, p
}

func (sm rcloneSMgr) JoinPath(d, f string) string {
	return d + "/" + f
}

func (sm localSMgr) JoinPath(d, f string) string {
	return path.Join(d, f)
}

func (sm rcloneSMgr) LsDir(p string) ([]string, error) {
	catCmd := exec.Command(rcloneBinary, "lsjson", "--no-modtime", "--no-mimetype", "--fast-list", "--max-depth", "1", "--files-only", p)
	catOut, err := catCmd.Output()
	if err != nil {
		return nil, err
	}
	var recs []rcloneLsRecord
	if err := json.Unmarshal(catOut, &recs); err != nil {
		return nil, err
	}
	var files []string
	for _, r := range recs {
		files = append(files, r.Path)
	}
	return files, nil
}

func (sm localSMgr) LsDir(p string) ([]string, error) {
	files, err := ioutil.ReadDir(p)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range files {
		if f.Mode().IsRegular() {
			names = append(names, f.Name())
		}
	}
	return names, nil
}

type rcloneLsRecord struct {
	Path string
	//Name string
	//Size int
	//ModTime string
	//IsDir bool
	//Tier string
}

func (sm rcloneSMgr) LsDir2(p string, f StorageMgrLsDir2Func) error {
	catCmd := exec.Command(rcloneBinary, "lsjson", "--no-modtime", "--no-mimetype", "--fast-list", "--max-depth", "2", "--files-only", p)
	catOut, err := catCmd.Output()
	if err != nil {
		return err
	}
	var recs []rcloneLsRecord
	if err := json.Unmarshal(catOut, &recs); err != nil {
		return err
	}
	for _, r := range recs {
		ss := strings.Split(r.Path, "/")
		if len(ss) == 2 {
			f(ss[0], ss[1])
		}
	}
	return nil
}

func (sm localSMgr) LsDir2(p string, f StorageMgrLsDir2Func) error {
	l1, err := ioutil.ReadDir(p)
	if err != nil {
		return err
	}
	for _, d := range l1 {
		if d.Mode().IsDir() {
			l2, err := ioutil.ReadDir(path.Join(p, d.Name()))
			if err == nil {
				for _, x := range l2 {
					if x.Mode().IsRegular() {
						f(d.Name(), x.Name())
					}
				}
			}
		}
	}
	return nil
}

func (sm rcloneSMgr) ExistInDir(d, f string) (bool, error) {
	files, err := TheRcloneSMgr.LsDir(d)
	if err != nil {
		return false, err
	}
	for _, x := range files {
		if x == f {
			return true, nil
		}
	}
	return false, nil
}

func (sm localSMgr) ExistInDir(d, f string) (bool, error) {
	_, err := os.Lstat(path.Join(d, f))
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (sm rcloneSMgr) MkdirAll(p string) error {
	return nil
}

func (sm localSMgr) MkdirAll(p string) error {
	return os.MkdirAll(p, DEFAULT_DIR_PERM)
}

func (sm rcloneSMgr) ReadFile(p string) ([]byte, error) {
	catCmd := exec.Command(rcloneBinary, "cat", p)
	catOut, err := catCmd.Output()
	if err != nil {
		return nil, err
	}
	return catOut, nil
}

func (sm localSMgr) ReadFile(p string) ([]byte, error) {
	return ioutil.ReadFile(p)
}

func (sm rcloneSMgr) WriteFile(p string, d []byte) error {
	cmd := exec.Command(rcloneBinary, "rcat", p)
	cmdIn, _ := cmd.StdinPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := cmdIn.Write(d); err != nil {
		return err
	}
	if err := cmdIn.Close(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (sm localSMgr) WriteFile(p string, d []byte) error {
	tp := p + "-temp"
	out, err := os.OpenFile(tp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, DEFAULT_FILE_PERM)
	if err != nil {
		return err
	}
	_, err = out.Write(d)
	if err != nil {
		out.Close()
		os.Remove(tp)
		return err
	}
	err = out.Close()
	if err != nil {
		os.Remove(tp)
		return err
	}
	err = os.Rename(tp, p)
	if err != nil {
		os.Remove(tp)
		return err
	}
	return nil
}

func (sm rcloneSMgr) DeleteFile(p string) error {
	cmd := exec.Command(rcloneBinary, "deletefile", p)
	return cmd.Run()
}

func (sm localSMgr) DeleteFile(p string) error {
	return os.Remove(p)
}

func (sm rcloneSMgr) WriteLockFile(p string) error {
	d := time.Now().UTC().Format(time.RFC3339Nano) + "\n"
	tp := p + "/tmp-lock"
	err := TheRcloneSMgr.WriteFile(tp, []byte(d))
	if err != nil {
		return os.ErrExist
	}
	cmd := exec.Command(rcloneBinary, "moveto", tp, p)
	err = cmd.Run()
	if err != nil {
		fmt.Print("MOVE ERROR", err)
		return os.ErrExist
	}
	return nil
}

func (sm localSMgr) WriteLockFile(p string) error {
	lockFile, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, DEFAULT_FILE_PERM)
	if err != nil {
		return err
	}
	lockFile.Close()
	return nil
}

func (sm rcloneSMgr) RemoveLockFile(p string) error {
	TheRcloneSMgr.DeleteFile(p)
	TheRcloneSMgr.DeleteFile(p + "/tmp-lock")
	return nil
}

func (sm localSMgr) RemoveLockFile(p string) error {
	return os.Remove(p)
}