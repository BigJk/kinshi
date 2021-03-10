package kinshi

import (
	"fmt"
	"reflect"
	"sync"
)

type EntityID uint64

const (
	EntityNone = EntityID(0)
)

// Entity represents the basic form of a entity.
type Entity interface {
	ID() EntityID
	SetID(EntityID)
}

// BaseEntity is the base implementation of the
// Entity interface and should be embedded into
// your own structs to make it a entity.
type BaseEntity struct {
	id EntityID
}

// ID returns the assigned id of the entity
func (b *BaseEntity) ID() EntityID {
	return b.id
}

// SetID sets the id of the entity. This should not be
// used by a user as it is managed by the ECS.
func (b *BaseEntity) SetID(id EntityID) {
	b.id = id
}

// DynamicEntity is a special entity with the option
// to dynamically add and remove components.
type DynamicEntity interface {
	Entity
	SetComponent(interface{}) error
	RemoveComponent(interface{}) error
	GetComponent(string) (interface{}, error)
	HasComponent(interface{}) error
	GetComponents() []interface{}
}

// BaseDynamicEntity is the base implementation of the
// DynamicEntity interface and should be embedded into
// your own structs to make it a dynamic entity.
type BaseDynamicEntity struct {
	BaseEntity
	sync.Mutex
	components map[string]interface{}
}

// SetComponents sets or adds a component with the data of c.
func (b *BaseDynamicEntity) SetComponent(c interface{}) error {
	b.Lock()
	defer b.Unlock()

	if reflect.TypeOf(c).Kind() != reflect.Ptr {
		return fmt.Errorf("component needs to be passed as pointer")
	}

	if b.components == nil {
		b.components = map[string]interface{}{}
	}

	b.components[getTypeName(c)] = c
	return nil
}

// RemoveComponent removes a component of the type c.
func (b *BaseDynamicEntity) RemoveComponent(c interface{}) error {
	b.Lock()
	defer b.Unlock()

	if b.components == nil {
		b.components = map[string]interface{}{}
	}

	typeName := getTypeName(c)

	if _, ok := b.components[typeName]; ok {
		delete(b.components, typeName)
		return nil
	}

	return ErrNotFound
}

// GetComponent tries to fetch a component by name.
func (b *BaseDynamicEntity) GetComponent(t string) (interface{}, error) {
	b.Lock()
	defer b.Unlock()

	if b.components == nil {
		b.components = map[string]interface{}{}
	}

	if val, ok := b.components[t]; ok {
		return val, nil
	}

	return nil, ErrNotFound
}

// HasComponent checks if the entity has a certain component.
// If t is a string it will check if a component is present by name.
// If t is a struct or a pointer to a struct the name of the type
// will be used to check if the component is present.
func (b *BaseDynamicEntity) HasComponent(t interface{}) error {
	b.Lock()
	defer b.Unlock()

	if b.components == nil {
		b.components = map[string]interface{}{}
	}

	var typeName string
	switch t.(type) {
	case string:
		typeName = t.(string)
	default:
		typeName = getTypeName(t)
	}

	if _, ok := b.components[typeName]; ok {
		return nil
	}

	return ErrNotFound
}

// GetComponents returns a slice with all the component
// instances as interface{}.
func (b *BaseDynamicEntity) GetComponents() []interface{} {
	b.Lock()
	defer b.Unlock()

	if b.components == nil {
		b.components = map[string]interface{}{}
	}

	var comps []interface{}
	for _, v := range b.components {
		comps = append(comps, reflect.ValueOf(v).Interface())
	}

	return comps
}
