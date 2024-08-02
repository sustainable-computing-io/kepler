# validator

<!--toc:start-->
- [validator](#validator)
  - [Requirements](#requirements)
  - [Installation](#installation)
  - [Usage](#usage)
  - [License](#license)
<!--toc:end-->

[![PyPI - Version](https://img.shields.io/pypi/v/validator.svg)](https://pypi.org/project/validator)
[![PyPI - Python Version](https://img.shields.io/pypi/pyversions/validator.svg)](https://pypi.org/project/validator)

## Requirements

- Python 3.11 and above
- stress-ng on both the remote and local machines
- [Hatch](https://hatch.pypa.io/latest/install/) for development

## Installation

Navigate to the `validator` directory and run:

```sh
hatch shell
pip install .
```

## Usage

- Configure and Generate `validator.yaml` file

  - Create the `validator.yaml` file based on the
  [validator.yaml.sample](validator.yaml.sample) template provided.
  - Adjust the configuration according to your environment and requirements.

- Run the validator

After configuring the `validator.yaml` file, start the validator with:

```sh
validator stress
```

## License

`validator` is distributed under the terms of the
[Apache 2.0](http://www.apache.org/licenses/) license.
