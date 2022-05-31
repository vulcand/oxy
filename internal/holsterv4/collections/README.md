## Priority Queue
Provides a Priority Queue implementation as described [here](https://en.wikipedia.org/wiki/Priority_queue)

```go
queue := collections.NewPriorityQueue()

queue.Push(&collections.PQItem{
    Value: "thing3",
    Priority: 3,
})

queue.Push(&collections.PQItem{
    Value: "thing1",
    Priority: 1,
})

queue.Push(&collections.PQItem{
    Value: "thing2",
    Priority: 2,
})

// Pops item off the queue according to the priority instead of the Push() order
item := queue.Pop()

fmt.Printf("Item: %s", item.Value.(string))

// Output: Item: thing1
```
