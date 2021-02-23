package kinshi

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

type Health struct {
	Value int
	Max   int
}

type Pos struct {
	X int
	Y int
}

type Name struct {
	Value string
}

type Unit struct {
	BaseEntity
	Health
	Pos
	Name
}

type Velocity struct {
	X float64
	Y float64
}

type DynamicUnit struct {
	BaseDynamicEntity
	Name
}

func TestECS(t *testing.T) {
	ecs := New()

	nameMap := map[string]EntityID{}

	t.Run("AddEntity", func(t *testing.T) {
		// Insert 100 entities with static components
		for i := 0; i < 100; i++ {
			id, err := ecs.AddEntity(&Unit{
				Health: Health{
					Value: 100,
					Max:   150,
				},
				Pos: Pos{
					X: 0,
					Y: 0,
				},
				Name: Name{
					Value: fmt.Sprint(i),
				},
			})
			nameMap[fmt.Sprint(i)] = id
			assert.NoError(t, err, "entity insertion failed")
		}

		// Insert 50 components with static and dynamically added components
		for i := 0; i < 50; i++ {
			dynUnit := DynamicUnit{
				Name: Name{
					Value: fmt.Sprintf("DynamicUnit %d", i),
				},
			}
			assert.NoError(t, dynUnit.SetComponent(Velocity{
				X: 0.5,
				Y: 0.1,
			}), "dynamic component insertion failed")

			id, err := ecs.AddEntity(&dynUnit)
			nameMap[dynUnit.Name.Value] = id
			assert.NoError(t, err, "entity insertion failed")
		}

		// Check that counts match
		assert.Equal(t, 100, ecs.IterateSpecific(Unit{}).Count(), "unit count doesn't match")
		assert.Equal(t, 150, ecs.Iterate(Name{}).Count(), "unit count doesn't match")
		assert.Equal(t, 50, ecs.IterateSpecific(DynamicUnit{}).Count(), "unit count doesn't match")
		assert.Equal(t, 50, ecs.Iterate(Velocity{}).Count(), "unit count doesn't match")
	})

	t.Run("Iterate", func(t *testing.T) {
		for _, ent := range ecs.Iterate(Name{}) {
			assert.NoError(t, ent.View(func(n *Name) {
				assert.Equal(t, nameMap[n.Value], ent.GetEntity().ID(), "entity id doesn't match")
			}), "failed while view")
		}
	})

	t.Run("View", func(t *testing.T) {
		for _, ent := range ecs.Iterate(Name{}) {
			// Change name value
			assert.NoError(t, ent.View(func(n *Name) {
				n.Value += " CHANGED"
			}), "failed while view")

			// Check if change is stored
			assert.NoError(t, ecs.MustGet(ent.GetEntity().ID()).View(func(n *Name) {
				assert.True(t, strings.HasSuffix(n.Value, "CHANGED"), "change wasn't observed")
			}), "failed while view")
		}
	})

	t.Run("ViewSpecific", func(t *testing.T) {
		for i, ent := range ecs.IterateSpecific(DynamicUnit{}) {
			// Change name value
			assert.NoError(t, ent.ViewSpecific(func(unit *DynamicUnit) {
				unit.Name.Value = fmt.Sprint(i)

				assert.NoError(t, unit.HasComponent(Velocity{}), "dynamic unit is missing a component")
			}), "failed while view")

			// Check if change is stored
			assert.NoError(t, ecs.MustGet(ent.GetEntity().ID()).View(func(n *Name) {
				assert.Equal(t, fmt.Sprint(i), n.Value, "change wasn't observed")
			}), "failed while view")
		}
	})
}

func BenchmarkECS_AddEntity(b *testing.B) {
	ecs := New()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = ecs.AddEntity(&Unit{
			Health: Health{
				Value: 100,
				Max:   150,
			},
			Pos: Pos{
				X: 0,
				Y: 0,
			},
			Name: Name{
				Value: "name",
			},
		})
	}
}

func BenchmarkECS_Iterate(b *testing.B) {
	runForN := func(n int, g int, b *testing.B) {
		ecs := New()
		ecs.SetRoutineCount(g)

		for i := 0; i < n/2; i++ {
			_, _ = ecs.AddEntity(&Unit{
				Health: Health{
					Value: 100,
					Max:   150,
				},
				Pos: Pos{
					X: 0,
					Y: 0,
				},
				Name: Name{
					Value: fmt.Sprint(i),
				},
			})
			_, _ = ecs.AddEntity(&DynamicUnit{
				Name: Name{
					Value: "name",
				},
			})
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			ecs.Iterate(Health{}, Pos{}, Name{})
		}
	}

	// 1 go routines allowed

	b.Run("1-100", func(b *testing.B) {
		runForN(100, 1, b)
	})

	b.Run("1-1000", func(b *testing.B) {
		runForN(1000, 1, b)
	})

	b.Run("1-10000", func(b *testing.B) {
		runForN(10000, 1, b)
	})

	b.Run("1-100000", func(b *testing.B) {
		runForN(100000, 1, b)
	})

	b.Run("1-1000000", func(b *testing.B) {
		runForN(1000000, 1, b)
	})

	b.Run("2-100", func(b *testing.B) {
		runForN(100, 2, b)
	})

	// 2 go routines allowed

	b.Run("2-1000", func(b *testing.B) {
		runForN(1000, 2, b)
	})

	b.Run("2-10000", func(b *testing.B) {
		runForN(10000, 2, b)
	})

	b.Run("2-100000", func(b *testing.B) {
		runForN(100000, 2, b)
	})

	b.Run("2-1000000", func(b *testing.B) {
		runForN(1000000, 2, b)
	})

	// 4 go routines allowed

	b.Run("4-100", func(b *testing.B) {
		runForN(100, 4, b)
	})

	b.Run("4-1000", func(b *testing.B) {
		runForN(1000, 4, b)
	})

	b.Run("4-10000", func(b *testing.B) {
		runForN(10000, 4, b)
	})

	b.Run("4-100000", func(b *testing.B) {
		runForN(100000, 4, b)
	})

	b.Run("4-1000000", func(b *testing.B) {
		runForN(1000000, 4, b)
	})
}

func BenchmarkECS_View(b *testing.B) {
	ecs := New()

	id, _ := ecs.AddEntity(&Unit{
		Health: Health{
			Value: 100,
			Max:   150,
		},
		Pos: Pos{
			X: 0,
			Y: 0,
		},
		Name: Name{
			Value: "test",
		},
	})

	wrap := ecs.MustGet(id)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := wrap.View(func(h *Health, p *Pos) {})
		if err != nil {
			b.Fatal(err)
		}
	}
}
