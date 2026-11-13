package handlers

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"open-veth/internal/models"
)

type resourceRemoverStub struct {
	calls  []string
	failAt string
}

func (s *resourceRemoverStub) DeleteNode(_ context.Context, id string) error {
	s.calls = append(s.calls, "container:"+id)
	if s.failAt == "container" {
		return errors.New("Docker unavailable")
	}
	return nil
}

func (s *resourceRemoverStub) RemoveImage(_ context.Context, image string) error {
	s.calls = append(s.calls, "image:"+image)
	if s.failAt == "image" {
		return errors.New("image in use")
	}
	return nil
}

func TestRemoveNodeResources(t *testing.T) {
	for _, stage := range []string{"", "container", "image"} {
		t.Run(stage, func(t *testing.T) {
			remover := &resourceRemoverStub{failAt: stage}
			node := models.Node{ID: "n1", Name: "router", ContainerID: "container-id", SnapshotImage: "snapshot"}
			err := removeNodeResources(context.Background(), remover, node)
			if (err != nil) != (stage != "") {
				t.Fatalf("unexpected error: %v", err)
			}
			want := []string{"container:container-id", "image:snapshot"}
			if stage == "container" {
				want = want[:1]
			}
			if !reflect.DeepEqual(remover.calls, want) {
				t.Fatalf("unexpected cleanup order: %v", remover.calls)
			}
		})
	}
}

func TestRemoveNodeResourcesWithoutRuntime(t *testing.T) {
	remover := &resourceRemoverStub{}
	if err := removeNodeResources(context.Background(), remover, models.Node{Name: "router"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(remover.calls, []string{"container:router"}) {
		t.Fatalf("must find the container by name: %v", remover.calls)
	}
}
