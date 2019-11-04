package data

import (
	"context"
	"io/ioutil"

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
	fs.MkdirAll("/repo", os.ModeDir)
	repo, _ := fs.Chroot("/repo")
	dot, _ := repo.Chroot("/.git")
	s := filesystem.NewStorage(dot, nil)

	r, err := git.Init(s, repo)
	assert.NilError(t, err)
	w, err := r.Worktree()
	assert.NilError(t, err)
	// err = w.Checkout(&git.CheckoutOptions{
	// 	Branch: plumbing.NewBranchReferenceName("master"),
	// 	Force:  true,
	// })
	// assert.NilError(t, err)
	repo.MkdirAll("/foo/bar", os.ModeDir)
	f, err := repo.Create("/foo/bar/hi.txt")
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
	// make the repo dirty
	f.Write([]byte("dirty file"))

	// set up a bare repo
	fs.MkdirAll("/bare.git", os.ModeDir)
	fs.MkdirAll("/barewt", os.ModeDir)
	repo, _ = fs.Chroot("/barewt")
	dot, _ = fs.Chroot("/bare.git")
	s = filesystem.NewStorage(dot, nil)

	r, err = git.Init(s, repo)
	assert.NilError(t, err)

	w, err = r.Worktree()
	assert.NilError(t, err)

	f, err = repo.Create("/hello.txt")
	assert.NilError(t, err)
	f.Write([]byte("hello world"))
	w.Add(f.Name())
	_, err = w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	assert.NilError(t, err)

	return fs
}

func TestOpenFileRepo(t *testing.T) {
	ctx := context.TODO()
	repoFS := setupGitRepo(t)
	g := gitsource{}

	overrideFSLoader(repoFS)
	defer overrideFSLoader(gitroot)

	fs, repo, err := g.openFileRepo(ctx, mustParseURL("git+file:///repo"))
	assert.NilError(t, err)

	f, err := fs.Open("/foo/bar/hi.txt")
	assert.NilError(t, err)
	b, _ := ioutil.ReadAll(f)
	assert.Equal(t, "hello world", string(b))

	ref, err := repo.Reference(plumbing.NewBranchReferenceName("master"), true)
	assert.NilError(t, err)
	assert.Equal(t, "refs/heads/master", ref.Name().String())
}

func TestOpenBareFileRepo(t *testing.T) {
	ctx := context.TODO()
	repoFS := setupGitRepo(t)
	g := gitsource{}

	overrideFSLoader(repoFS)
	defer overrideFSLoader(gitroot)

	fs, _, err := g.openFileRepo(ctx, mustParseURL("git+file:///bare.git"))
	assert.NilError(t, err)

	f, err := fs.Open("/hello.txt")
	assert.NilError(t, err)
	b, _ := ioutil.ReadAll(f)
	assert.Equal(t, "hello world", string(b))
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

	u := mustParseURL(gompURL + "#3.4.x")
	_, repo, err := g.openHTTPRepo(ctx, u)
	assert.NilError(t, err)
	ref, err := repo.Head()
	assert.NilError(t, err)
	assert.Equal(t, "refs/heads/3.4.x", ref.Name().String())

	// tag := "v3.5.0"
	// _, repo, err = g.openHTTPRepo(ctx, mustParseURL(gompURL+"#refs/tags/"+tag))
	// assert.NilError(t, err)
	// titer, err := repo.Tags()
	// assert.NilError(t, err)
	// err = titer.ForEach(func(ref *plumbing.Reference) error {
	// 	// tref, err := repo.Tag("refs/tags/"+tag)
	// 	// t.Logf("tag: %#v", ref)
	// 	if ref.Name().Short() == tag {
	// 		head, err := repo.Head()
	// 		if err != nil {
	// 			return err
	// 		}
	// 		assert.Equal(t, ref.Hash(), head.Hash())
	// 	}
	// 	return nil
	// })
	// assert.NilError(t, err)
}

// type dummyClient struct {
// 	fs billy.Filesystem
// 	s  storage.Storer
// }

// func (c *dummyClient) NewUploadPackSession(*transport.Endpoint, transport.AuthMethod) (
// 	transport.UploadPackSession, error) {
// 	s := &upSession{session{
// 		storer: c.s,
// 	}}
// 	return s, nil
// }

// func (c *dummyClient) NewReceivePackSession(*transport.Endpoint, transport.AuthMethod) (
// 	transport.ReceivePackSession, error) {
// 	return nil, nil
// }

// type session struct {
// 	storer   storage.Storer
// 	caps     *capability.List
// 	asClient bool
// }

// func (s *session) Close() error {
// 	return nil
// }

// func (s *session) checkSupportedCapabilities(cl *capability.List) error {
// 	for _, c := range cl.All() {
// 		if !s.caps.Supports(c) {
// 			return fmt.Errorf("unsupported capability: %s", c)
// 		}
// 	}

// 	return nil
// }

// type upSession struct {
// 	session
// }

// func (s *upSession) AdvertisedReferences() (*packp.AdvRefs, error) {
// 	ar := packp.NewAdvRefs()

// 	if err := s.setSupportedCapabilities(ar.Capabilities); err != nil {
// 		return nil, err
// 	}

// 	s.caps = ar.Capabilities

// 	if err := setReferences(s.storer, ar); err != nil {
// 		return nil, err
// 	}

// 	if err := setHEAD(s.storer, ar); err != nil {
// 		return nil, err
// 	}

// 	if s.asClient && len(ar.References) == 0 {
// 		return nil, transport.ErrEmptyRemoteRepository
// 	}

// 	return ar, nil
// }

// func (s *upSession) UploadPack(ctx context.Context, req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
// 	if req.IsEmpty() {
// 		return nil, transport.ErrEmptyUploadPackRequest
// 	}

// 	if err := req.Validate(); err != nil {
// 		return nil, err
// 	}

// 	if s.caps == nil {
// 		s.caps = capability.NewList()
// 		if err := s.setSupportedCapabilities(s.caps); err != nil {
// 			return nil, err
// 		}
// 	}

// 	if err := s.checkSupportedCapabilities(req.Capabilities); err != nil {
// 		return nil, err
// 	}

// 	s.caps = req.Capabilities

// 	if len(req.Shallows) > 0 {
// 		return nil, fmt.Errorf("shallow not supported")
// 	}

// 	objs, err := s.objectsToUpload(req)
// 	if err != nil {
// 		return nil, err
// 	}

// 	pr, pw := io.Pipe()
// 	e := packfile.NewEncoder(pw, s.storer, false)
// 	go func() {
// 		// TODO: plumb through a pack window.
// 		_, err := e.Encode(objs, 10)
// 		pw.CloseWithError(err)
// 	}()

// 	return packp.NewUploadPackResponseWithPackfile(req,
// 		gitioutil.NewContextReadCloser(ctx, pr),
// 	), nil
// }

// func (s *upSession) objectsToUpload(req *packp.UploadPackRequest) ([]plumbing.Hash, error) {
// 	haves, err := revlist.Objects(s.storer, req.Haves, nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return revlist.Objects(s.storer, req.Wants, haves)
// }

// func (*upSession) setSupportedCapabilities(c *capability.List) error {
// 	if err := c.Set(capability.Agent, capability.DefaultAgent); err != nil {
// 		return err
// 	}

// 	if err := c.Set(capability.OFSDelta); err != nil {
// 		return err
// 	}

// 	return nil
// }

// type rpSession struct {
// 	session
// }

// func setHEAD(s storer.Storer, ar *packp.AdvRefs) error {
// 	ref, err := s.Reference(plumbing.HEAD)
// 	if err == plumbing.ErrReferenceNotFound {
// 		return nil
// 	}

// 	if err != nil {
// 		return err
// 	}

// 	if ref.Type() == plumbing.SymbolicReference {
// 		if err := ar.AddReference(ref); err != nil {
// 			return nil
// 		}

// 		ref, err = storer.ResolveReference(s, ref.Target())
// 		if err == plumbing.ErrReferenceNotFound {
// 			return nil
// 		}

// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if ref.Type() != plumbing.HashReference {
// 		return plumbing.ErrInvalidType
// 	}

// 	h := ref.Hash()
// 	ar.Head = &h

// 	return nil
// }

// func setReferences(s storer.Storer, ar *packp.AdvRefs) error {
// 	//TODO: add peeled references.
// 	iter, err := s.IterReferences()
// 	if err != nil {
// 		return err
// 	}

// 	return iter.ForEach(func(ref *plumbing.Reference) error {
// 		if ref.Type() != plumbing.HashReference {
// 			return nil
// 		}

// 		ar.References[ref.Name().String()] = ref.Hash()
// 		return nil
// 	})
// }

// func referenceExists(s storer.ReferenceStorer, n plumbing.ReferenceName) (bool, error) {
// 	_, err := s.Reference(n)
// 	if err == plumbing.ErrReferenceNotFound {
// 		return false, nil
// 	}

// 	return err == nil, err
// }
