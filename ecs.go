package kinshi

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrNoID          = errors.New("not id")
	ErrAlreadyExists = errors.New("already exists")
)

type ECS struct {
	sync.Mutex
	idCounter uint64
	entities  map[EntityID]Entity
}

// New creates a new instance of a ECS
func New() *ECS {
	return &ECS{
		entities: map[EntityID]Entity{},
	}
}

func (ecs *ECS) nextId() EntityID {
	ecs.Lock()
	defer ecs.Unlock()

	ecs.idCounter += 1
	return EntityID(ecs.idCounter)
}

// AddEntity adds a Entity to the ECS storage and
// returns the assigned EntityID.
func (ecs *ECS) AddEntity(ent Entity) (EntityID, error) {
	if ent.ID() == 0 {
		ent.SetID(ecs.nextId())
	}

	ecs.Lock()
	defer ecs.Unlock()

	if _, ok := ecs.entities[ent.ID()]; ok {
		return ent.ID(), ErrAlreadyExists
	}

	ecs.entities[ent.ID()] = ent
	return ent.ID(), nil
}

// RemoveEntity removes a Entity from the ECS storage.
func (ecs *ECS) RemoveEntity(ent Entity) error {
	if ent.ID() == 0 {
		return ErrNoID
	}

	ecs.Lock()
	defer ecs.Unlock()

	if _, ok := ecs.entities[ent.ID()]; ok {
		delete(ecs.entities, ent.ID())
		return nil
	}

	return ErrNotFound
}

// EntityWrap is a wrapper for Entity that provides functions
// to get a view into the Entity components.
type EntityWrap struct {
	ent Entity
}

// GetEntity returns the wrapped Entity.
func (ew *EntityWrap) GetEntity() Entity {
	return ew.ent
}

// View calls fn with pointer to requested components. If you modify
// the data inside the fn function the changes will be copied to the
// Entity after the function returns. If fn returns a error and the
// returned error is != nil the changes to the data will not be copied
// to the Entity.
//
// For example you want to get a view on the Pos{} and Velocity{} struct:
//    ew.View(func(p *Pos, v *Velocity) {
//    	p.X += v.X
//    	p.Y += v.Y
//    })
func (ew *EntityWrap) View(fn interface{}) error {
	if reflect.TypeOf(fn).Kind() != reflect.Func {
		return fmt.Errorf("fn not function")
	}

	fnType := reflect.TypeOf(fn)
	var callInstances []reflect.Value

	for i := 0; i < fnType.NumIn(); i++ {
		ptr := reflect.New(fnType.In(i).Elem())

		if err := findType(ew.ent, ptr.Interface()); err != nil {
			if dyn, ok := ew.ent.(DynamicEntity); ok && dyn.HasComponent(ptr.Interface()) == nil && dyn.GetComponent(ptr.Interface()) == nil {
			} else {
				return err
			}
		}

		callInstances = append(callInstances, ptr)
	}

	res := reflect.ValueOf(fn).Call(callInstances)
	if len(res) == 1 {
		if res[0].Interface() != nil {
			err := res[0].Interface().(error)
			if err != nil {
				return err
			}
		}
	}

	for i := range callInstances {
		if err := setType(ew.ent, callInstances[i].Interface()); err != nil {
			if dyn, ok := ew.ent.(DynamicEntity); ok && dyn.HasComponent(callInstances[i].Interface()) == nil && dyn.SetComponent(callInstances[i].Interface()) == nil {
			} else {
				return err
			}
		}
	}

	return nil
}

// ViewSpecific calls fn with pointer to the specific requested struct.
// Its like fetching a named Entity. Changes to the struct data directly
// applies to the backing Entity and there is no rollback like in View.
//
// For example you want to get a view on the Player{} Entity struct:
//    ew.ViewSpecific(func(p *Player) {
//    	fmt.Println(p.Name)
//    })
func (ew *EntityWrap) ViewSpecific(fn interface{}) error {
	if reflect.TypeOf(fn).Kind() != reflect.Func {
		return fmt.Errorf("fn not function")
	}

	fnType := reflect.TypeOf(fn)
	if fnType.NumIn() != 1 {
		return fmt.Errorf("fn needs a single argument")
	}

	reflect.ValueOf(fn).Call([]reflect.Value{reflect.ValueOf(ew.ent)})

	return nil
}

// Valid checks if the wrapped Entity is valid (and present).
func (ew *EntityWrap) Valid() bool {
	return ew.ent.ID() != EntityNone
}

type EntityIterator []*EntityWrap

// Count returns the number of found entities.
func (it EntityIterator) Count() int {
	return len(it)
}

// Iterate searches for entities that contain all the given types and returns
// a iterator that can be range'd over.
//
// For example you want to get fetch all entities containing a
// Pos{} and Velocity{} component:
//    for _, ew := range ecs.Iterate(Pos{}, Velocity{}) {
//        // Work with the EntityWrap
//    }
func (ecs *ECS) Iterate(types ...interface{}) EntityIterator {
	var foundEnts []*EntityWrap

	for _, v := range ecs.entities {
		allFound := true
		for i := range types {
			if err := hasType(v, types[i]); err != nil {
				if dyn, ok := v.(DynamicEntity); ok && dyn.HasComponent(types[i]) == nil {

				} else {
					allFound = false
					break
				}
			}
		}
		if allFound {
			foundEnts = append(foundEnts, &EntityWrap{ent: v})
		}
	}

	return foundEnts
}

// IterateSpecific searches for entities of a named type and returns
// a iterator that can be range'd over.
//
// For example you want to get fetch all entities that are of
// the Player Entity type:
//    for _, ew := range ecs.IterateSpecific(Player{}) {
//        // Work with the EntityWrap
//    }
func (ecs *ECS) IterateSpecific(t interface{}) EntityIterator {
	var foundEnts []*EntityWrap

	searchName := getTypeName(t)

	for _, v := range ecs.entities {
		if getTypeName(v) == searchName {
			foundEnts = append(foundEnts, &EntityWrap{ent: v})
		}
	}

	return foundEnts
}

// IterateID returns a iterator that can be range'd over for
// the given Entity ids.
func (ecs *ECS) IterateID(ids ...EntityID) EntityIterator {
	var foundEnts []*EntityWrap

	for i := range ids {
		if v, ok := ecs.entities[ids[i]]; ok {
			foundEnts = append(foundEnts, &EntityWrap{ent: v})
		}
	}

	return foundEnts
}

// Get fetches a Entity by id.
func (ecs *ECS) Get(id EntityID) (*EntityWrap, error) {
	if v, ok := ecs.entities[id]; ok {
		return &EntityWrap{ent: v}, nil
	}
	return nil, ErrNotFound
}

// MustGet fetches a Entity by id but won't return a error
// if not found.
func (ecs *ECS) MustGet(id EntityID) *EntityWrap {
	w, _ := ecs.Get(id)
	return w
}

// Access creates a EntityWrap for a given Entity so that
// the data of the Entity can be accessed in a convenient way.
func (ecs *ECS) Access(ent Entity) *EntityWrap {
	return &EntityWrap{ent: ent}
}
