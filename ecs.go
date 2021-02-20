package ecs

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

// AddEntity adds a entity to the ECS storage
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

// RemoveEntity removes a entity from the ECS storage
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

type EntityWrap struct {
	ent Entity
}

// GetEntity returns the wrapped entity
func (ew *EntityWrap) GetEntity() Entity {
	return ew.ent
}

// View calls fn with pointer to requested components
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

// ViewSpecific will call fn with a pointer to the entity struct
// instead of the components
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

// Valid checks if the wrapped entity is valid (and present)
func (ew *EntityWrap) Valid() bool {
	return ew.ent.ID() != EntityNone
}

type EntityIterator []*EntityWrap

func (it EntityIterator) Count() int {
	return len(it)
}

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

func (ecs *ECS) IterateID(ids ...EntityID) EntityIterator {
	var foundEnts []*EntityWrap

	for i := range ids {
		if v, ok := ecs.entities[ids[i]]; ok {
			foundEnts = append(foundEnts, &EntityWrap{ent: v})
		}
	}

	return foundEnts
}

func (ecs *ECS) Get(id EntityID) (*EntityWrap, error) {
	if v, ok := ecs.entities[id]; ok {
		return &EntityWrap{ent: v}, nil
	}
	return nil, ErrNotFound
}

func (ecs *ECS) MustGet(id EntityID) *EntityWrap {
	w, _ := ecs.Get(id)
	return w
}

func (ecs *ECS) Access(ent Entity) *EntityWrap {
	return &EntityWrap{ent: ent}
}
