# RabbitMQ Message Router
Pyongo routes RabbitMQ deliveries to handler functions by the routing key of deliveries. Middleware functions, modifiers and wildcards make routing flexible and easy.

## Usage
Create a new handler that will route messages. The constructor supports a list of options, but default values are applied if not set.
```golang
// Use default settings
def := pyongo.New()

// Use custom settings
hndl := pyongo.New(
    pyongo.ConcurrencyOption(5),
    pyongo.DefaultLoggerOption(),
)
```
Delivery routing keys get tokenized by `.` characters. You can set handler functions to tokens, or create groups.
```golang
// Handling deliveries with key "public.events.created"
pubGrp := hndl.Group("public")
evnGrp := pubGrp.Group("events")
evnGrp.Set("created", func(pctx *pyongo.Context) {
    msg := pctx.GetMessage()
    fmt.Println(string(msg.Body))
    msg.Ack(false)
})
```
Groups or handlers with key `*` capture all tokens which have not been captured by a specific token. Attaching a handler with key `#` on the base handler will capture all messages that failed to route. Deliveries that fail to route are acknowledged by default, so it is recommended to set up the global handler.
```golang
// Handling deliveries with key "books.created", "books.updated".. etc
bkGrp := hndl.Group("books")
bkGrp.Set("*", func(pctx *pyongo.Context) {
    pctx.GetMessage().Ack(false)
})

// Handling any deliveries that did not route
hndl.Set("#", func(pctx *pyongo.Context) {
    pctx.GetMessage().Nack(false, false)
})
```

You can add middlewares to groups or handlers that execute in the order they are defined in. In the middleware, use `Next()` on the context to progress to the next function.
```golang
bkGrp := hndl.Group("books")
bkGrp.Use(func(pctx *pyongo.Context) {
    start := time.Now()
    pctx.Next()
    fmt.Println("Completed in: ", time.Since(start))
})
bkGrp.Set("created", func(pctx *pyongo.Context) {
    pctx.GetMessage().Ack(false)
})
```

Once handlers are configured, start processing the deliveries. 
```golang
if err := hndl.Start(); err != nil {
    panic(err)
}
```

To feed deliveries to the handler, you can submit them manually or attach a channel that the handler will read deliveries from.
```golang
// Submit deliveries manually
var dlv amqp.Delivery
if err := hndl.Submit(context.TODO(), dlv); err != nil {
    panic(err)
}

// Attach a channel to read deliveries from
dlvCh := make(chan amqp.Delivery)
if err := hndl.AddSource(dlvCh); err != nil {
    panic(err)
}
```
Closing the handler will immediately stop listening to the attached channels, but will wait for the submitted deliveries to finish handling before closing.
```golang
err := hndl.Close();
```


