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

type typeMeta struct {
	t      reflect.Type
	fields map[string]struct{}
}

type ECS struct {
	sync.Mutex
	idCounter uint64
	entities  map[EntityID]Entity
	metaCache map[string]typeMeta
}

// New creates a new instance of a ECS
func New() *ECS {
	return &ECS{
		entities:  map[EntityID]Entity{},
		metaCache: map[string]typeMeta{},
	}
}

func (ecs *ECS) nextId() EntityID {
	ecs.Lock()
	defer ecs.Unlock()

	ecs.idCounter += 1
	return EntityID(ecs.idCounter)
}

func (ecs *ECS) cacheType(ent Entity) {
	tn := getTypeName(ent)
	if _, ok := ecs.metaCache[tn]; ok {
		return
	}

	t := reflect.TypeOf(ent).Elem()
	ecs.metaCache[tn] = typeMeta{
		t:      t,
		fields: map[string]struct{}{},
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() == reflect.Struct {
			ecs.metaCache[tn].fields[field.Name] = struct{}{}
		}
	}
}

// AddEntity adds a Entity to the ECS storage and
// returns the assigned EntityID.
func (ecs *ECS) AddEntity(ent Entity) (EntityID, error) {
	if reflect.TypeOf(ent).Kind() != reflect.Ptr {
		return EntityNone, fmt.Errorf("please pass your entity as pointer")
	}

	if ent.ID() == 0 {
		ent.SetID(ecs.nextId())
	}

	ecs.Lock()
	defer ecs.Unlock()

	ecs.cacheType(ent)

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
	parent *ECS
	ent    Entity
}

// GetEntity returns the wrapped Entity.
func (ew *EntityWrap) GetEntity() Entity {
	return ew.ent
}

// View calls fn with pointer to requested components. If you change
// any data it will directly modify the Entity data. The pointer that
// the fn functions is called with are pointing straight to the components.
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

	ew.parent.Lock()
	defer ew.parent.Unlock()

	for i := 0; i < fnType.NumIn(); i++ {
		compName := fnType.In(i).Elem().Name()

		ptr, err := fetchPtrOfType(ew.ent, compName)
		if err != nil {
			if dyn, ok := ew.ent.(DynamicEntity); ok {
				ptr, err := dyn.GetComponent(compName)
				if err != nil {
					return err
				}
				callInstances = append(callInstances, reflect.ValueOf(ptr))
			} else {
				return err
			}
		}

		callInstances = append(callInstances, reflect.ValueOf(ptr))
	}

	res := reflect.ValueOf(fn).Call(callInstances)

	// If the user supplied function returns a error return it
	if len(res) == 1 {
		if res[0].Interface() != nil {
			err := res[0].Interface().(error)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// ViewSpecific calls fn with pointer to the specific requested struct.
// Its like fetching a named Entity. Changes to the struct data directly
// applies to the Entity.
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

	ew.parent.Lock()
	defer ew.parent.Unlock()

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
	ecs.Lock()
	defer ecs.Unlock()

	var foundEnts []*EntityWrap

	for _, v := range ecs.entities {
		allFound := true
		for i := range types {
			if val, ok := ecs.metaCache[getTypeName(v)]; ok {
				if _, ok := val.fields[getTypeName(types[i])]; ok {
					continue
				}
			}

			if dyn, ok := v.(DynamicEntity); ok && dyn.HasComponent(types[i]) == nil {

			} else {
				allFound = false
				break
			}
		}
		if allFound {
			foundEnts = append(foundEnts, &EntityWrap{parent: ecs, ent: v})
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
	ecs.Lock()
	defer ecs.Unlock()

	var foundEnts []*EntityWrap

	searchName := getTypeName(t)

	for _, v := range ecs.entities {
		if getTypeName(v) == searchName {
			foundEnts = append(foundEnts, &EntityWrap{parent: ecs, ent: v})
		}
	}

	return foundEnts
}

// IterateID returns a iterator that can be range'd over for
// the given Entity ids.
func (ecs *ECS) IterateID(ids ...EntityID) EntityIterator {
	ecs.Lock()
	defer ecs.Unlock()

	var foundEnts []*EntityWrap

	for i := range ids {
		if v, ok := ecs.entities[ids[i]]; ok {
			foundEnts = append(foundEnts, &EntityWrap{parent: ecs, ent: v})
		}
	}

	return foundEnts
}

// Get fetches a Entity by id.
func (ecs *ECS) Get(id EntityID) (*EntityWrap, error) {
	ecs.Lock()
	defer ecs.Unlock()

	if v, ok := ecs.entities[id]; ok {
		return &EntityWrap{parent: ecs, ent: v}, nil
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
	return &EntityWrap{parent: ecs, ent: ent}
}
