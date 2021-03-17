package kinshi

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"io"
	"reflect"
	"sort"
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

type serializedEntity struct {
	ID         EntityID
	Type       string
	Components map[string]interface{}
}

type entityEntry struct {
	TypeName string `json:"type_name"`
	Ent      Entity `json:"ent"`
}

type ECS struct {
	sync.RWMutex
	idCounter     uint64
	entities      []entityEntry
	metaCache     map[string]typeMeta
	compMetaCache map[string]reflect.Type
	routines      int
}

// New creates a new instance of a ECS
func New() *ECS {
	return &ECS{
		entities:      []entityEntry{},
		metaCache:     map[string]typeMeta{},
		compMetaCache: map[string]reflect.Type{},
		routines:      1,
	}
}

func (ecs *ECS) nextId() EntityID {
	ecs.Lock()
	defer ecs.Unlock()

	ecs.idCounter += 1
	return EntityID(ecs.idCounter)
}

func (ecs *ECS) cacheComponent(name string, t reflect.Type) {
	ecs.compMetaCache[name] = t
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
			ecs.cacheComponent(field.Type.Name(), field.Type)
		}
	}
}

func (ecs *ECS) findEntity(id EntityID) (*entityEntry, int, bool) {
	l := len(ecs.entities)
	found := sort.Search(l, func(i int) bool {
		return ecs.entities[i].Ent.ID() >= id
	})
	if found == l {
		return nil, 0, false
	}
	return &ecs.entities[found], found, true
}

// Unmarshal reads a JSON encoded ECS snapshot and loads
// all the entities from it. The inner storage will be overwritten
// so all entities that have been added before will be deleted.
//
// Important: If you want to serialize dynamic entities you need
// to register all possible components with RegisterComponent()
// before!
func (ecs *ECS) Unmarshal(reader io.Reader) error {
	ecs.Lock()
	defer ecs.Unlock()

	var ses []serializedEntity

	dec := json.NewDecoder(reader)
	if err := dec.Decode(&ses); err != nil {
		return err
	}

	ecs.entities = []entityEntry{}

	for i := range ses {
		ent := entityEntry{
			TypeName: ses[i].Type,
			Ent:      nil,
		}

		if meta, ok := ecs.metaCache[ses[i].Type]; ok {
			newInstance := reflect.New(meta.t)

			for comp, val := range ses[i].Components {
				field := newInstance.Elem().FieldByName(comp)
				if field.IsValid() {
					if err := mapstructure.Decode(val, field.Addr().Interface()); err != nil {
						// TODO: Error handling
						continue
					}
				} else {
					if dyn, ok := newInstance.Interface().(DynamicEntity); ok {
						if compType, ok := ecs.compMetaCache[comp]; ok {
							newComponent := reflect.New(compType)

							if err := mapstructure.Decode(val, newComponent.Interface()); err != nil {
								// TODO: Error handling
								continue
							}

							_ = dyn.SetComponent(newComponent.Interface())
						}
					}
				}
			}

			ent.Ent = newInstance.Interface().(Entity)
			ent.Ent.SetID(ses[i].ID)
			ecs.entities = append(ecs.entities, ent)
		}
	}

	if len(ecs.entities) > 0 {
		ecs.idCounter = uint64(ecs.entities[len(ecs.entities)-1].Ent.ID()) + 1
	} else {
		ecs.idCounter = 0
	}

	return nil
}

// Marshal encodes all entities into JSON.
func (ecs *ECS) Marshal(writer io.Writer) error {
	ecs.Lock()
	defer ecs.Unlock()

	var ses []serializedEntity
	for i := range ecs.entities {
		se := serializedEntity{
			ID:         ecs.entities[i].Ent.ID(),
			Type:       ecs.entities[i].TypeName,
			Components: map[string]interface{}{},
		}

		val := reflect.ValueOf(ecs.entities[i].Ent).Elem()
		for j := 0; j < val.NumField(); j++ {
			name := val.Type().Field(j).Name
			if name == "BaseEntity" || name == "BaseDynamicEntity" {
				continue
			}

			field := val.Field(j)
			if field.Kind() != reflect.Struct {
				continue
			}

			se.Components[name] = field.Interface()
		}

		if dyn, ok := ecs.entities[i].Ent.(DynamicEntity); ok {
			comps := dyn.GetComponents()
			for i := range comps {
				se.Components[getTypeName(comps[i])] = comps[i]
			}
		}

		ses = append(ses, se)
	}

	enc := json.NewEncoder(writer)
	enc.SetIndent("", "\t")
	return enc.Encode(ses)
}

// RegisterEntity caches information about a entity.
func (ecs *ECS) RegisterEntity(ent Entity) {
	ecs.cacheType(ent)
}

// RegisterComponent caches information about components
// this is needed if you want to serialize dynamic entities
// as the reflection information needs to be available
// before the unmarshal.
func (ecs *ECS) RegisterComponent(c interface{}) {
	if reflect.ValueOf(c).Kind() == reflect.Ptr {
		ecs.cacheComponent(getTypeName(c), reflect.TypeOf(c).Elem())
	} else {
		ecs.cacheComponent(getTypeName(c), reflect.TypeOf(c))
	}
}

// SetRoutineCount sets the number of go routines
// that are allowed to spawn to parallelize searches
// over the entities.
func (ecs *ECS) SetRoutineCount(n int) {
	ecs.Lock()
	defer ecs.Unlock()

	ecs.routines = n
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

	if _, _, ok := ecs.findEntity(ent.ID()); ok {
		return ent.ID(), ErrAlreadyExists
	}

	ecs.entities = append(ecs.entities, entityEntry{
		TypeName: getTypeName(ent),
		Ent:      ent,
	})
	return ent.ID(), nil
}

// RemoveEntity removes a Entity from the ECS storage.
func (ecs *ECS) RemoveEntity(ent Entity) error {
	if ent.ID() == 0 {
		return ErrNoID
	}

	ecs.Lock()
	defer ecs.Unlock()

	if _, id, ok := ecs.findEntity(ent.ID()); ok {
		ecs.entities = append(ecs.entities[:id], ecs.entities[id+1:]...)
		ent.SetID(EntityNone)
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

	ew.parent.RLock()
	defer ew.parent.RUnlock()

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
				continue
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
			err, ok := res[0].Interface().(error)
			if ok && err != nil {
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
//        fmt.Println(p.Name)
//    })
func (ew *EntityWrap) ViewSpecific(fn interface{}) error {
	if reflect.TypeOf(fn).Kind() != reflect.Func {
		return fmt.Errorf("fn not function")
	}

	fnType := reflect.TypeOf(fn)
	if fnType.NumIn() != 1 {
		return fmt.Errorf("fn needs a single argument")
	}

	ew.parent.RLock()
	defer ew.parent.RUnlock()

	res := reflect.ValueOf(fn).Call([]reflect.Value{reflect.ValueOf(ew.ent)})

	// If the user supplied function returns a error return it
	if len(res) == 1 {
		if res[0].Interface() != nil {
			err, ok := res[0].Interface().(error)
			if ok && err != nil {
				return err
			}
		}
	}

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
	ecs.RLock()
	defer ecs.RUnlock()

	wg := sync.WaitGroup{}
	mtx := sync.Mutex{}

	wg.Add(ecs.routines)

	var foundEnts []*EntityWrap

	step := len(ecs.entities)/ecs.routines + 1
	for w := 0; w < ecs.routines; w++ {
		go func(start int, l int) {
			var localFoundEnts []*EntityWrap

			for i := start; i < start+l && i < len(ecs.entities); i++ {
				allFound := true
				for j := range types {
					if val, ok := ecs.metaCache[ecs.entities[i].TypeName]; ok {
						if _, ok := val.fields[getTypeName(types[j])]; ok {
							continue
						}
					}

					if dyn, ok := ecs.entities[i].Ent.(DynamicEntity); ok && dyn.HasComponent(types[j]) == nil {

					} else {
						allFound = false
						break
					}
				}
				if allFound {
					localFoundEnts = append(localFoundEnts, &EntityWrap{parent: ecs, ent: ecs.entities[i].Ent})
				}
			}

			mtx.Lock()
			foundEnts = append(foundEnts, localFoundEnts...)
			mtx.Unlock()

			wg.Done()
		}(step*w, step)
	}

	wg.Wait()

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
	ecs.RLock()
	defer ecs.RUnlock()

	wg := sync.WaitGroup{}
	mtx := sync.Mutex{}

	wg.Add(ecs.routines)

	var foundEnts []*EntityWrap

	searchName := getTypeName(t)
	step := len(ecs.entities)/ecs.routines + 1
	for w := 0; w < ecs.routines; w++ {
		var localFoundEnts []*EntityWrap

		go func(start int, l int) {
			for i := start; i < start+l && i < len(ecs.entities); i++ {
				if ecs.entities[i].TypeName == searchName {
					foundEnts = append(foundEnts, &EntityWrap{parent: ecs, ent: ecs.entities[i].Ent})
				}
			}

			mtx.Lock()
			foundEnts = append(foundEnts, localFoundEnts...)
			mtx.Unlock()

			wg.Done()
		}(step*w, step)
	}

	wg.Wait()

	return foundEnts
}

// IterateID returns a iterator that can be range'd over for
// the given Entity ids.
func (ecs *ECS) IterateID(ids ...EntityID) EntityIterator {
	ecs.RLock()
	defer ecs.RUnlock()

	var foundEnts []*EntityWrap

	for i := range ids {
		if v, _, ok := ecs.findEntity(ids[i]); ok {
			foundEnts = append(foundEnts, &EntityWrap{parent: ecs, ent: v.Ent})
		}
	}

	return foundEnts
}

// Get fetches a Entity by id.
func (ecs *ECS) Get(id EntityID) (*EntityWrap, error) {
	ecs.RLock()
	defer ecs.RUnlock()

	if v, _, ok := ecs.findEntity(id); ok {
		return &EntityWrap{parent: ecs, ent: v.Ent}, nil
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
