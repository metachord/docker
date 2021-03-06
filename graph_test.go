package docker

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	// Root should exist
	if _, err := os.Stat(graph.Root); err != nil {
		t.Fatal(err)
	}
	// All() should be empty
	if l, err := graph.All(); err != nil {
		t.Fatal(err)
	} else if len(l) != 0 {
		t.Fatalf("List() should return %d, not %d", 0, len(l))
	}
}

// FIXME: Do more extensive tests (ex: create multiple, delete, recreate;
//       create multiple, check the amount of images and paths, etc..)
func TestGraphCreate(t *testing.T) {
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image, err := graph.Create(archive, nil, "Testing")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateId(image.Id); err != nil {
		t.Fatal(err)
	}
	if image.Comment != "Testing" {
		t.Fatalf("Wrong comment: should be '%s', not '%s'", "Testing", image.Comment)
	}
	if images, err := graph.All(); err != nil {
		t.Fatal(err)
	} else if l := len(images); l != 1 {
		t.Fatalf("Wrong number of images. Should be %d, not %d", 1, l)
	}
}

func TestRegister(t *testing.T) {
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image := &Image{
		Id:      GenerateId(),
		Comment: "testing",
		Created: time.Now(),
	}
	err = graph.Register(archive, image)
	if err != nil {
		t.Fatal(err)
	}
	if images, err := graph.All(); err != nil {
		t.Fatal(err)
	} else if l := len(images); l != 1 {
		t.Fatalf("Wrong number of images. Should be %d, not %d", 1, l)
	}
	if resultImg, err := graph.Get(image.Id); err != nil {
		t.Fatal(err)
	} else {
		if resultImg.Id != image.Id {
			t.Fatalf("Wrong image ID. Should be '%s', not '%s'", image.Id, resultImg.Id)
		}
		if resultImg.Comment != image.Comment {
			t.Fatalf("Wrong image comment. Should be '%s', not '%s'", image.Comment, resultImg.Comment)
		}
	}
}

func TestMount(t *testing.T) {
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image, err := graph.Create(archive, nil, "Testing")
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempDir("", "docker-test-graph-mount-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	rootfs := path.Join(tmp, "rootfs")
	if err := os.MkdirAll(rootfs, 0700); err != nil {
		t.Fatal(err)
	}
	rw := path.Join(tmp, "rw")
	if err := os.MkdirAll(rw, 0700); err != nil {
		t.Fatal(err)
	}
	if err := image.Mount(rootfs, rw); err != nil {
		t.Fatal(err)
	}
	// FIXME: test for mount contents
	defer func() {
		if err := Unmount(rootfs); err != nil {
			t.Error(err)
		}
	}()
}

func TestDelete(t *testing.T) {
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 0)
	img, err := graph.Create(archive, nil, "Bla bla")
	if err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 1)
	if err := graph.Delete(img.Id); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 0)

	// Test 2 create (same name) / 1 delete
	img1, err := graph.Create(archive, nil, "Testing")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = graph.Create(archive, nil, "Testing"); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 2)
	if err := graph.Delete(img1.Id); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 1)

	// Test delete wrong name
	if err := graph.Delete("Not_foo"); err == nil {
		t.Fatalf("Deleting wrong ID should return an error")
	}
	assertNImages(graph, t, 1)

	// Test delete twice (pull -> rm -> pull -> rm)
	if err := graph.Register(archive, img1); err != nil {
		t.Fatal(err)
	}
	if err := graph.Delete(img1.Id); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 1)
}

func assertNImages(graph *Graph, t *testing.T, n int) {
	if images, err := graph.All(); err != nil {
		t.Fatal(err)
	} else if actualN := len(images); actualN != n {
		t.Fatalf("Expected %d images, found %d", n, actualN)
	}
}

/*
 * HELPER FUNCTIONS
 */

func tempGraph(t *testing.T) *Graph {
	tmp, err := ioutil.TempDir("", "docker-graph-")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := NewGraph(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func testArchive(t *testing.T) Archive {
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	return archive
}

func fakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)
		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}
