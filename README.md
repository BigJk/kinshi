# kinshi

[![Documentation](https://godoc.org/github.com/BigJk/kinshi?status.svg)](http://godoc.org/github.com/BigJk/kinshi)

While working on the architecture of a [7DRL](https://7drl.com/) game I noticed there wasn't any small and dynamic ECS implementations in go that fitted my bill. I came up with my own small solution, which I dubbed "kinshin" (禁止, japanese for 'forbidden'). Why forbidden? Because this ECS is build on reflection magic and should probably not be used in any serious projects. You lose a lot of compile time checks that way and it probably isn't great performance wise, but for my use case for a small turn based roguelike that doesn't matter.

## Get It

```
go get github.com/BigJk/kinshi
```

## Example

```go
package main

import (
	"fmt"
	"github.com/BigJk/kinshi"
)

// Components
type Health struct {
	Value int
	Max   int
}

type Pos struct {
	X float64
	Y float64
}

type Velocity struct {
	X float64
	Y float64
}

// Entity
type Unit struct {
	kinshi.BaseEntity // Required for entities
	Velocity
	Health
	Pos
}

func main() {
	ecs := kinshi.New()

	// Insert some entities
	for i := 0; i < 100; i++ {
		_, _ = ecs.AddEntity(&Unit{
			Velocity: Velocity{},
			Health: Health{
				Value: 100,
				Max:   100,
			},
			Pos: Pos{
				X: 5,
				Y: 10,
			},
		})
	}

	// Iterate over all entities that contain a pos and velocity component
	for _, ent := range ecs.Iterate(Pos{}, Velocity{}) {
		// Access the data of the wanted components that the entity contains
		_ = ent.View(func(p *Pos, v *Velocity) {
			fmt.Printf("[ent=%d] x=%.2f y=%.2f vx=%.2f vy=%.2f\n", ent.GetEntity().ID(), p.X, p.Y, v.X, v.Y)

			// p and v point directly to the components that are inside the entity!
		})
	}
  
	// Iterate over all entities that are exactly a unit.
	// Other entities with the same components are NOT contained
	// in this query.
	for _, ent := range ecs.IterateSpecific(Unit{}) {
		// Instead of getting the component data you can also access the entity type directly
		_ = ent.ViewSpecific(func(u *Unit) {
			fmt.Printf("[ent=%d] x=%.2f y=%.2f vx=%.2f vy=%.2f\n", ent.GetEntity().ID(), u.Pos.X, u.Pos.Y, u.Velocity.X, u.Velocity.Y)

			// u points directly to the entity!
		})
	}
}
```

### Dynamic Entity

You can choose to either use "static" entities that have fixed pre-defined components just like in the above example or you can choose to use dynamic entities. You can even mix and match.

```go

type DynamicUnit struct {
	kinshi.BaseDynamicEntity // Required for dynamic entities
	Health                   // Fixed static component
}

func addDynamicEntity(ecs *kinshi.ECS) {
	dynu := DynamicUnit{
		Health: Health{
			Value: 100,
			Max:   100,
		},
	}

	dynu.SetComponent(Pos{
		X: 25,
		Y: 20,
	})

	dynu.SetComponent(Velocity{
		X: 25,
		Y: 20,
	})

	ecs.AddEntity(&dynu)
}
```

### Systems

The way you handle systems is not part of kinshi. A system can be as simple as a function that takes a pointer to a ECS instance, some additional game state and performs some modifications on the entities and game state. The simplest system for adding the velocity to the entities position could look like that:

```go
func SystemMovement(ecs *kinshi.ECS) {
	for _, ent := range ecs.Iterate(Pos{}, Velocity{}) {
		ent.View(func(p *Pos, v *Velocity) {
			p.X += v.X
			p.Y += v.Y
		})
	}
}
```
