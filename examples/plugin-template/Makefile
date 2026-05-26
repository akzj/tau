.PHONY: build clean test

NAME=hello-plugin

build:
	go build -ldflags="-s -w" -o $(NAME) .

test:
	echo '{"method":"tools","params":{}}' | ./$(NAME)
	echo '{"method":"execute","params":{"tool":"hello","args":{"name":"tau"}}}' | ./$(NAME)

clean:
	rm -f $(NAME)
