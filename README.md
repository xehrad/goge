<p align="center">
  <img src="assets/logo/goge_logo.png" alt="Goge Logo" width="256"/>
</p>

# goge


**goge** is a code generator for the [Fiber](https://github.com/gofiber/fiber) framework. 
It automatically creates API handlers and OpenAPI specs from annotated Go functions.


- Annotate your Fiber handler with a comment:

  ```go
  type PingParams struct {
      ID    string `gogeUrl:"id"`              // read from the URL path
      Auth  string `gogeHeader:"Authorization"`// read from HTTP header
      Query string `gogeQuery:"filter"`        // read from the query String
      Page  int    `gogeQuery:"page"`          // read from the query Int
      Name  string `json:"name"`               // from POST/PATCH/PUT body
  }

  //goge:api method=POST path=/ping/:id
  func Ping(c *fiber.Ctx, params *PingParams) (*PingResp, error) {
      return &PingResp{ID: params.ID}, nil
  }
  ```


- After running `go generate ./...` goge generates a handler and router automatically:

  ```go
  func PingHandler(c *fiber.Ctx) error {
    req := new(PingParams)
    c.BodyParser(req)

    req.ID = c.Params("id")
    req.Auth = c.Get("Authorization")
    req.Query = c.Query("filter")
    req.Page = c.QueryInt("page")

    res, _ := Ping(c, req)
    return c.JSON(res)
  }


  func GogeRouter(app *fiber.App) {
    app.Add("POST", "/ping/:id", PingHandler)
  }
  ```


- Inside your project, add the following to your `main.go`

    ```go
    //go:generate goge

    package main

    func main() {
      app := fiber.New()
      lib.GogeRouter(app)
      log.Fatal(app.Listen(":8080"))
    }
    ```

## Installation

1. Make sure you have **Go 1.24+** installed.

1. This will place the `goge` binary in your `GOPATH/bin` (by default `~/go/bin`)

    ```bash
    go install github.com/xehrad/goge@latest
    ````



1. Add `$GOPATH/bin` to your PATH. If not already configured, add this to your shell config (`~/.bashrc` or `~/.zshrc`):

    ```bash
    export PATH=$PATH:$(go env GOPATH)/bin
    ```

1. Reload your shell:

    ```bash
    source ~/.bashrc
    # or
    source ~/.zshrc
    ```
1. Now you should be able to run goge
    * A ready-to-use handler function at `lib/api_generated.go`
    * OpenAPI documentation wit params at `lib/openapi.json`

    ```bash
    goge [package] [src]

    # package:  is name of package, default is "lib"
    # src:      is path of src, default is "./lib"
    ```



## Name
**goge** — pronounced like **"Go Dje"** — comes from **Go + Generate**, reflecting its purpose of generating Go code automatically.  

In Persian, **goge** also means *tomato* 🍅.
