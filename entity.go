package ecs

import (
	"reflect"
	"sync"
)

type EntityID uint64

const (
	EntityNone = EntityID(0)
)

type Entity interface {
	ID() EntityID
	SetID(EntityID)
}

type BaseEntity struct {
	id EntityID
}

func (b *BaseEntity) ID() EntityID {
	return b.id
}

func (b *BaseEntity) SetID(id EntityID) {
	b.id = id
}

type DynamicEntity interface {
	Entity
	SetComponent(interface{}) error
	RemoveComponent(interface{}) error
	GetComponent(interface{}) error
	HasComponent(interface{}) error
	GetComponents() []interface{}
}

type BaseDynamicEntity struct {
	BaseEntity
	sync.Mutex
	components map[string]interface{}
}

func (b *BaseDynamicEntity) SetComponent(c interface{}) error {
	b.Lock()
	defer b.Unlock()

	if b.components == nil {
		b.components = map[string]interface{}{}
	}

	b.components[getTypeName(c)] = c
	return nil
}

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

func (b *BaseDynamicEntity) GetComponent(out interface{}) error {
	b.Lock()
	defer b.Unlock()

	if b.components == nil {
		b.components = map[string]interface{}{}
	}

	if val, ok := b.components[getTypeName(out)]; ok {
		reflect.ValueOf(out).Elem().Set(reflect.ValueOf(val))
		return nil
	}

	return ErrNotFound
}

func (b *BaseDynamicEntity) HasComponent(out interface{}) error {
	b.Lock()
	defer b.Unlock()

	if b.components == nil {
		b.components = map[string]interface{}{}
	}

	if _, ok := b.components[getTypeName(out)]; ok {
		return nil
	}

	return ErrNotFound
}

func (b *BaseDynamicEntity) GetComponents() []interface{} {
	b.Lock()
	defer b.Unlock()

	if b.components == nil {
		b.components = map[string]interface{}{}
	}

	var comps []interface{}
	for _, v := range b.components {
		comps = append(comps, v)
	}

	return comps
}
