# Generate UML diagrams

## 0. Install `goplantuml`

Install from
[https://github.com/jfeliu007/goplantuml](https://github.com/jfeliu007/goplantuml)

## 1. For each golang package generate `plantumls` using the following command

```bash
goplantuml --recursive -show-compositions -show-aggregations path/to/pkg >./package.plantuml
```
<!-- markdownlint-disable  MD013 -->
<!-- Teporarily disable MD013 - Line length for the title below  -->

## 2. Use [plantuml](https://plantuml.com/download) to convert planuml files to `svg` files

<!-- markdownlint-enable  MD013 -->
NeoVim plugin `neovim-soil` was used to generate svg files from plantuml files
