package ecs

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
