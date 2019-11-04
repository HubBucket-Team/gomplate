package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hairyhenderson/gomplate/base64"
	"github.com/hairyhenderson/gomplate/env"

	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/server"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

// Parses the path (if any) out of the URL.
// It's delimited by a '//'
func parseGitPath(u *url.URL) (*url.URL, string, error) {
	if u == nil {
		return nil, "", fmt.Errorf("parseGitPath: no url provided (%v)", u)
	}
	parts := strings.SplitN(u.Path, "//", 2)
	switch len(parts) {
	case 1:
		return u, "/", nil
	case 2:
		path := "/" + parts[1]
		// copy the input url so we can modify it
		out, _ := url.Parse(u.String())

		i := strings.LastIndex(out.Path, path)
		out.Path = out.Path[:i]
		return out, path, nil
	}
	return nil, "", fmt.Errorf("parseGitPath: inconceivable error")
}

// gitroot - default filesystem
var gitroot = osfs.New("/")

func overrideFSLoader(fs billy.Filesystem) {
	l := server.NewFilesystemLoader(fs)
	client.InstallProtocol("file", server.NewClient(l))
}

func readGit(source *Source, args ...string) ([]byte, error) {
	ctx := context.Background()
	u := source.URL
	ref := u.Fragment
	repoURL, path, err := parseGitPath(u)
	if err != nil {
		return nil, err
	}
	fmt.Println(ref)

	g := gitsource{}

	var fs billy.Filesystem
	switch u.Scheme {
	case "git+file":
		fs, _, err = g.openFileRepo(ctx, repoURL)
		if err != nil {
			return nil, err
		}
	case "git+http", "git+https":
		fs, _, err = g.openHTTPRepo(ctx, repoURL)
		if err != nil {
			return nil, err
		}
	case "git+ssh":
		fs, _, err = g.openSSHRepo(ctx, repoURL)
		if err != nil {
			return nil, err
		}
	case "git":
		fs, _, err = g.openGitRepo(ctx, repoURL)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("scheme %s cannot be handled by git datasource support", u.Scheme)
	}

	mimeType, out, err := g.read(fs, path)
	if mimeType != "" {
		source.mediaType = mimeType
	}
	return out, err
}

type gitsource struct {
}

// clone an HTTP(S) repo for later reading. u must be the URL to the repo
// itself, and must have any file path stripped
func (g gitsource) openHTTPRepo(ctx context.Context, u *url.URL) (billy.Filesystem, *git.Repository, error) {
	fs := memfs.New()
	storer := memory.NewStorage()

	auth, err := g.auth(u)
	if err != nil {
		return nil, nil, err
	}

	scheme := strings.TrimLeft(u.Scheme, "git+")
	u.Scheme = scheme

	var ref plumbing.ReferenceName
	if strings.HasPrefix(u.Fragment, "refs/") {
		ref = plumbing.ReferenceName(u.Fragment)
	} else if u.Fragment != "" {
		ref = plumbing.NewBranchReferenceName(u.Fragment)
	} else {
		ref = plumbing.Master
	}
	u.Fragment = ""

	repo, err := git.CloneContext(ctx, storer, fs, &git.CloneOptions{
		URL:           u.String(),
		Auth:          auth,
		Depth:         1,
		ReferenceName: ref,
		SingleBranch:  true,
		Tags:          git.NoTags,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("git clone for %v failed: %w", u, err)
	}
	return fs, repo, nil
}

// clone an SSH repo for later reading. u must be the URL to the repo
// itself, and must have any file path stripped
func (g gitsource) openSSHRepo(ctx context.Context, u *url.URL) (billy.Filesystem, *git.Repository, error) {
	fs := memfs.New()
	storer := memory.NewStorage()

	auth, err := g.auth(u)
	if err != nil {
		return nil, nil, err
	}

	scheme := strings.TrimLeft(u.Scheme, "git+")
	u.Scheme = scheme

	var ref plumbing.ReferenceName
	if strings.HasPrefix(u.Fragment, "refs/") {
		ref = plumbing.ReferenceName(u.Fragment)
	} else if u.Fragment != "" {
		ref = plumbing.NewBranchReferenceName(u.Fragment)
	} else {
		ref = plumbing.Master
	}
	u.Fragment = ""

	repo, err := git.CloneContext(ctx, storer, fs, &git.CloneOptions{
		URL:           u.String(),
		Auth:          auth,
		Depth:         1,
		ReferenceName: ref,
		SingleBranch:  true,
		Tags:          git.NoTags,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("git clone for %v failed: %w", u, err)
	}
	return fs, repo, nil
}

func (g gitsource) openGitRepo(ctx context.Context, u *url.URL) (billy.Filesystem, *git.Repository, error) {
	fs := memfs.New()
	storer := memory.NewStorage()

	auth, err := g.auth(u)
	if err != nil {
		return nil, nil, err
	}

	scheme := strings.TrimLeft(u.Scheme, "git+")
	u.Scheme = scheme

	var ref plumbing.ReferenceName
	if strings.HasPrefix(u.Fragment, "refs/") {
		ref = plumbing.ReferenceName(u.Fragment)
	} else if u.Fragment != "" {
		ref = plumbing.NewBranchReferenceName(u.Fragment)
	} else {
		ref = plumbing.Master
	}
	u.Fragment = ""

	repo, err := git.CloneContext(ctx, storer, fs, &git.CloneOptions{
		URL:           u.String(),
		Auth:          auth,
		Depth:         1,
		ReferenceName: ref,
		SingleBranch:  true,
		Tags:          git.NoTags,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("git clone for %v failed: %w", u, err)
	}
	return fs, repo, nil
}

func (g gitsource) openFileRepo(ctx context.Context, u *url.URL) (billy.Filesystem, *git.Repository, error) {
	// repo := u.Path
	// fs, err := rootFS.Chroot(repo)
	// if err != nil {
	// 	return nil, nil, fmt.Errorf("chroot failed: %w", err)
	// }
	// dot, err := fs.Chroot(".git")
	// storer := filesystem.NewStorage(dot, nil)

	// r, err := git.Open(storer, fs)
	// if err != nil {
	// 	return nil, nil, fmt.Errorf("failed to open repo at %s: %w", repo, err)
	// }

	fs := memfs.New()
	storer := memory.NewStorage()
	auth, err := g.auth(u)
	if err != nil {
		return nil, nil, err
	}

	scheme := strings.TrimLeft(u.Scheme, "git+")
	u.Scheme = scheme

	var ref plumbing.ReferenceName
	if strings.HasPrefix(u.Fragment, "refs/") {
		ref = plumbing.ReferenceName(u.Fragment)
	} else if u.Fragment != "" {
		ref = plumbing.NewBranchReferenceName(u.Fragment)
	} else {
		ref = plumbing.Master
	}
	u.Fragment = ""

	repo, err := git.CloneContext(ctx, storer, fs, &git.CloneOptions{
		URL:  u.String(),
		Auth: auth,
		// Depth:         1,
		ReferenceName: ref,
		SingleBranch:  true,
		Tags:          git.NoTags,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("git clone for %v failed: %w", u, err)
	}
	return fs, repo, nil
}

// read - reads the provided path out of a git repo
func (g gitsource) read(fs billy.Filesystem, path string) (string, []byte, error) {
	fi, err := fs.Stat(path)
	if err != nil {
		return "", nil, fmt.Errorf("can't stat %s: %w", path, err)
	}
	if fi.IsDir() || strings.HasSuffix(path, string(filepath.Separator)) {
		out, err := g.readDir(fs, path)
		return jsonArrayMimetype, out, err
	}

	f, err := fs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return "", nil, fmt.Errorf("can't open %s: %w", path, err)
	}

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", nil, fmt.Errorf("can't read %s: %w", path, err)
	}

	return "", b, nil
}

func (g gitsource) readDir(fs billy.Filesystem, path string) ([]byte, error) {
	names, err := fs.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't read dir %s: %w", path, err)
	}
	files := make([]string, len(names))
	for i, v := range names {
		files[i] = v.Name()
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(files); err != nil {
		return nil, err
	}
	b := buf.Bytes()
	// chop off the newline added by the json encoder
	return b[:len(b)-1], nil
}

/*
auth methods:
- ssh named key (no password support)
	- GIT_SSH_KEY (base64-encoded) or GIT_SSH_KEY_FILE (base64-encoded, or not)
- ssh agent auth (preferred)
- http basic auth (for github, gitlab, bitbucket tokens)
- http token auth (bearer token, somewhat unusual)
*/
func (g gitsource) auth(u *url.URL) (auth transport.AuthMethod, err error) {
	user := u.User.Username()
	switch u.Scheme {
	case "git+http", "git+https":
		if pass := env.Getenv("GIT_HTTP_PASSWORD"); pass != "" {
			auth = &http.BasicAuth{Username: user, Password: pass}
		} else if tok := env.Getenv("GIT_HTTP_TOKEN"); tok != "" {
			// note docs on TokenAuth - this is rarely to be used
			auth = &http.TokenAuth{Token: tok}
		}
	case "git+ssh":
		k := env.Getenv("GIT_SSH_KEY")
		if k != "" {
			key, err := base64.Decode(k)
			if err != nil {
				key = []byte(k)
			}
			auth, err = ssh.NewPublicKeys(user, key, "")
		} else {
			auth, err = ssh.NewSSHAgentAuth(user)
		}
	}
	return auth, err
}
