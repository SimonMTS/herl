# Live reloading example

With `herl`, `entr`, `make`, and `go` installed, run `make dev` in this
directory. This will start the `herl` proxy, and setup `entr` to listen to
changes in `.go` and `.html` files.

Now open the proxy in your browser, `herl` outputs the url on startup:
`herl: proxy listening on: 127.0.0.1:3030`.
If you wanted this page to open automatically you could add `xdg-open
http://127.0.0.1:3030` to the make file (on linux at least).

You can now edit `example.go` or `index.html` in your editor of choice (or
using `echo` as in the demo.gif). And when you save either of the files `entr`
will restart the application, and `herl` will reload the browser.

![demo](./demo.gif)

