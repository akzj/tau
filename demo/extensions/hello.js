/// name: "hello"
/// description: "Example extension: logs turn events and registers a command"

tau.on("turn:start", function(event, ctx) {
    console.log("Turn started: " + event.turnID);
});

tau.on("tool:start", function(event, ctx) {
    console.log("Tool called: " + event.toolName);
});

tau.registerCommand("hello", {
    description: "Say hello",
    handler: function(args, ctx) {
        ctx.sendMessage("Hello from extension! Args: " + args);
    }
});

tau.registerTool({
    name: "hello-tool",
    description: "A sample tool registered by the hello extension",
    schema: {
        type: "object",
        properties: {
            name: { type: "string" }
        }
    },
    handler: function(params, ctx) {
        return "Hello, " + (params.name || "world") + "!";
    }
});
