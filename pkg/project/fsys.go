package project

import (
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FsysOp uint8

const (
	FsysOpCreate FsysOp = iota + 1
	FsysOpWrite
	FsysOpRemove
	FsysOpRename
	// FsysOpChmod // not used
)

type Fsys struct {
	watcher *fsnotify.Watcher
	out     chan FsysEvent
	done    chan struct{}
}

type FsysEvent struct {
	Path     string
	Op       FsysOp
	Time     time.Time
	IsDir    bool
	FileInfo os.FileInfo
}

func newFsys() (*Fsys, error) {

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fsys := &Fsys{
		watcher: fw,
		out:     make(chan FsysEvent, 1024),
		done:    make(chan struct{}),
	}

	go fsys.loop()
	return fsys, nil
}

func (fsys *Fsys) Events() <-chan FsysEvent {
	return fsys.out
}

func (fsys *Fsys) Close() error {
	close(fsys.done)
	return fsys.watcher.Close()
}

func (fsys *Fsys) loop() {
	defer close(fsys.out)

	for {
		select {
		case e, ok := <-fsys.watcher.Events:
			if !ok {
				return
			}
			fsys.handle(e)

		case <-fsys.watcher.Errors:
			// opcional: expor canal de erro; aqui ignorado para manter contrato limpo

		case <-fsys.done:
			return
		}
	}
}

func (fsys *Fsys) handle(e fsnotify.Event) {
	path := e.Name

	// tenta stat para saber se é dir (pode falhar em Remove)
	info, _ := os.Stat(path)
	isDir := info != nil && info.IsDir()

	if isDir {
		if e.Op&fsnotify.Create == fsnotify.Create {
			// recursion dinâmica: diretório criado
			_ = fsys.addRecursive(path)

		} else if e.Op&fsnotify.Rename == fsnotify.Rename {
			// A watch will be automatically removed if the watched path is deleted or
			// renamed. The exception is the Windows backend, which doesn't remove the
			// watcher on renames.
			_ = fsys.watcher.Remove(path)
		}
	}

	// normalização de op
	var ops []FsysOp

	if e.Op&fsnotify.Create == fsnotify.Create {
		ops = append(ops, FsysOpCreate)
	}
	if e.Op&fsnotify.Write == fsnotify.Write {
		ops = append(ops, FsysOpWrite)
	}
	if e.Op&fsnotify.Remove == fsnotify.Remove {
		ops = append(ops, FsysOpRemove)
	}
	if e.Op&fsnotify.Rename == fsnotify.Rename {
		ops = append(ops, FsysOpRename)
	}

	now := time.Now()

	for _, op := range ops {
		fsys.out <- FsysEvent{
			Path:     path,
			Op:       op,
			Time:     now,
			IsDir:    isDir,
			FileInfo: info,
		}
	}
}

func (fsys *Fsys) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignora erro local
		}
		if d.IsDir() {
			_ = fsys.watcher.Add(path)
		}
		return nil
	})
}
