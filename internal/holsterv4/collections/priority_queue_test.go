/*
Copyright 2017 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package collections_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vulcand/oxy/v2/internal/holsterv4/collections"
)

func toPtr(i int) any {
	return &i
}

func toInt(i any) int {
	return *(i.(*int))
}

func TestPeek(t *testing.T) {
	mh := collections.NewPriorityQueue()

	el := &collections.PQItem{
		Value:    toPtr(1),
		Priority: 5,
	}

	mh.Push(el)
	assert.Equal(t, 1, toInt(mh.Peek().Value))
	assert.Equal(t, 1, mh.Len())

	el = &collections.PQItem{
		Value:    toPtr(2),
		Priority: 1,
	}
	mh.Push(el)
	assert.Equal(t, 2, mh.Len())
	assert.Equal(t, 2, toInt(mh.Peek().Value))
	assert.Equal(t, 2, toInt(mh.Peek().Value))
	assert.Equal(t, 2, mh.Len())

	el = mh.Pop()

	assert.Equal(t, 2, toInt(el.Value))
	assert.Equal(t, 1, mh.Len())
	assert.Equal(t, 1, toInt(mh.Peek().Value))

	mh.Pop()
	assert.Equal(t, 0, mh.Len())
}

func TestUpdate(t *testing.T) {
	mh := collections.NewPriorityQueue()
	x := &collections.PQItem{
		Value:    toPtr(1),
		Priority: 4,
	}
	y := &collections.PQItem{
		Value:    toPtr(2),
		Priority: 3,
	}
	z := &collections.PQItem{
		Value:    toPtr(3),
		Priority: 8,
	}
	mh.Push(x)
	mh.Push(y)
	mh.Push(z)
	assert.Equal(t, 2, toInt(mh.Peek().Value))

	mh.Update(z, 1)
	assert.Equal(t, 3, toInt(mh.Peek().Value))

	mh.Update(x, 0)
	assert.Equal(t, 1, toInt(mh.Peek().Value))
}

func ExampleNewPriorityQueue() {
	queue := collections.NewPriorityQueue()

	queue.Push(&collections.PQItem{
		Value:    "thing3",
		Priority: 3,
	})

	queue.Push(&collections.PQItem{
		Value:    "thing1",
		Priority: 1,
	})

	queue.Push(&collections.PQItem{
		Value:    "thing2",
		Priority: 2,
	})

	// Pops item off the queue according to the priority instead of the Push() order
	item := queue.Pop()

	fmt.Printf("Item: %s", item.Value.(string))

	// Output: Item: thing1
}
