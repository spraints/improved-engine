# HTTP/2 client and server

This project includes a simple Go HTTP client and server that talk over HTTP/2. I want to use this to figure out what happens with various types of connection problems.

- Normal: all requests go over the same connection.

- Fewer "max streams" on server than threads on client: more connections get opened.

- Server process exits - client errors.

- `kill -STOP` server - ping + read-idle timeouts both elapse and then the client times out.

- Network connection drops - :question: I should be able to test this either with two computers or with a two-VM Vagrant.
