package data

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"gotest.tools/v3/assert"
)

func TestParseGitPath(t *testing.T) {
	_, _, err := parseGitPath(nil)
	assert.ErrorContains(t, err, "")

	data := []struct {
		url        string
		repo, path string
	}{
		{"git+https://github.com/hairyhenderson/gomplate//docs-src/content/functions/aws.yml",
			"git+https://github.com/hairyhenderson/gomplate/",
			"/docs-src/content/functions/aws.yml"},
		{"git+ssh://github.com/hairyhenderson/gomplate.git",
			"git+ssh://github.com/hairyhenderson/gomplate.git",
			"/"},
		{"git://example.com/foo//file.txt#someref",
			"git://example.com/foo/#someref", "/file.txt"},
		{"git+file:///home/foo/repo//file.txt#someref",
			"git+file:///home/foo/repo/#someref", "/file.txt"},
		{"git+file:///repo",
			"git+file:///repo", "/"},
		{"git+file:///foo//foo",
			"git+file:///foo/", "/foo"},
	}

	for _, d := range data {
		u, _ := url.Parse(d.url)
		repo, path, err := parseGitPath(u)
		assert.NilError(t, err)
		assert.Equal(t, d.repo, repo.String())
		assert.Equal(t, d.path, path)
	}
}

func TestReadGit(t *testing.T) {
	s := &Source{
		Alias: "foo",
		URL:   mustParseURL("git+file:///gitrepo"),
	}
	_, err := readGit(s)
	assert.ErrorContains(t, err, "failed to open repo")

	// mfs := memfs.New()
	// gitroot = mfs
	// mfs.MkdirAll("/gitrepo/.git", os.ModeDir)

	// s = &Source{
	// 	Alias: "foo",
	// 	URL:   mustParseURL("git+file:///gitrepo"),
	// }
	// out, err := readGit(s)
	// assert.NilError(t, err)
	// assert.DeepEqual(t, []string{}, out)
}

func TestReadGitRepo(t *testing.T) {
	g := gitsource{}
	fs := memfs.New()
	s := memory.NewStorage()

	git.Open(s, fs)
	_, _, err := g.read(fs, "")
	assert.ErrorContains(t, err, "")

	r, _ := git.Init(s, fs)
	w, _ := r.Worktree()
	fs.MkdirAll("/foo/bar", os.ModeDir)
	f, _ := fs.Create("/foo/bar/hi.txt")
	f.Write([]byte("hello world"))
	w.Add(f.Name())
	w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	// w.Checkout(&git.CheckoutOptions{
	// 	Hash:   h,
	// 	Branch: plumbing.NewBranchReferenceName("foo"),
	// 	Create: true,
	// })
	// w.Move("/foo/bar/hi.txt", "/foo/bar/hello.txt")
	// w.Commit("renaming file", &git.CommitOptions{})

	_, out, err := g.read(fs, "/bogus")
	assert.ErrorContains(t, err, "can't stat /bogus")

	mtype, out, err := g.read(fs, "/")
	assert.NilError(t, err)
	assert.Equal(t, `["foo"]`, string(out))
	assert.Equal(t, jsonArrayMimetype, mtype)

	mtype, out, err = g.read(fs, "/foo/bar")
	assert.NilError(t, err)
	assert.Equal(t, `["hi.txt"]`, string(out))
	assert.Equal(t, jsonArrayMimetype, mtype)

	mtype, out, err = g.read(fs, "/foo/bar/hi.txt")
	assert.NilError(t, err)
	assert.Equal(t, `hello world`, string(out))
	assert.Equal(t, "", mtype)
}

func setupGitRepo(t *testing.T) billy.Filesystem {
	fs := memfs.New()
	dot, _ := fs.Chroot(".git")
	s := filesystem.NewStorage(dot, nil)

	r, err := git.Init(s, fs)
	assert.NilError(t, err)
	w, err := r.Worktree()
	assert.NilError(t, err)
	// err = w.Checkout(&git.CheckoutOptions{
	// 	Branch: plumbing.NewBranchReferenceName("master"),
	// 	Force:  true,
	// })
	// assert.NilError(t, err)
	fs.MkdirAll("/foo/bar", os.ModeDir)
	f, err := fs.Create("/foo/bar/hi.txt")
	assert.NilError(t, err)
	f.Write([]byte("hello world"))
	w.Add(f.Name())
	w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	return fs
}

func TestOpenFileRepo(t *testing.T) {
	fs := setupGitRepo(t)
	g := gitsource{}

	_, repo, err := g.openFileRepo(fs, mustParseURL("/"))
	assert.NilError(t, err)

	ref, err := repo.Reference(plumbing.NewBranchReferenceName("master"), true)
	assert.NilError(t, err)
	assert.Equal(t, "refs/heads/master", ref.Name().String())
}

func TestOpenHTTPRepo(t *testing.T) {
	ctx := context.TODO()
	g := gitsource{}

	gompURL := "git+https://github.com/hairyhenderson/gomplate.git"

	// _, repo, err := g.openHTTPRepo(ctx, mustParseURL(gompURL))
	// assert.NilError(t, err)
	// // ref, err := repo.Reference(plumbing.NewBranchReferenceName("master"), true)
	// ref, err := repo.Head()
	// assert.NilError(t, err)
	// assert.Equal(t, "refs/heads/master", ref.Name().String())

	// u = mustParseURL("git+https://github.com/hairyhenderson/gomplate.git#3.4.x")
	// _, repo, err = g.openHTTPRepo(ctx, u)
	// assert.NilError(t, err)
	// ref, err = repo.Head()
	// assert.NilError(t, err)
	// assert.Equal(t, "refs/heads/3.4.x", ref.Name().String())

	tag := "v3.5.0"
	_, repo, err := g.openHTTPRepo(ctx, mustParseURL(gompURL+"#refs/tags/"+tag))
	assert.NilError(t, err)
	titer, err := repo.Tags()
	assert.NilError(t, err)
	err = titer.ForEach(func(ref *plumbing.Reference) error {
		// tref, err := repo.Tag("refs/tags/"+tag)
		t.Logf("tag: %#v", ref)
		if ref.Name().Short() == tag {
			head, err := repo.Head()
			if err != nil {
				return err
			}
			assert.Equal(t, ref.Hash(), head.Hash())
		}
		return nil
	})
	assert.NilError(t, err)
}
