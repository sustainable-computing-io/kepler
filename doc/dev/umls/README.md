### 0. Install  `goplantuml`
Install from [https://github.com/jfeliu007/goplantuml](https://github.com/jfeliu007/goplantuml)

### 1. For each golang pakage generate `plantumls` using the following command
```
goplantuml --recursive -show-compositions -show-aggregations path/to/pkg > ./package.plantuml
```

### 2. Use [plantuml](https://plantuml.com/download) to convert planuml files  to `svg` files
NeoVim plugin `neovim-soil` was used to generate svg files from plantuml files
