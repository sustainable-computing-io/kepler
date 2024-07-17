# validator

[![PyPI - Version](https://img.shields.io/pypi/v/validator.svg)](https://pypi.org/project/validator)
[![PyPI - Python Version](https://img.shields.io/pypi/pyversions/validator.svg)](https://pypi.org/project/validator)

-----

**Table of Contents**

- [Installation](#installation)
- [License](#license)

## Requirements

- Python 3.11 and above.
- stress-ng on both the remote and local machines.

## Installation

Inside of the `validator` directory:
```console
pip install .
```

## Usage

Configure Prometheus jobs. The one to scrape the metal metrics is assumed to be named `metal` and the one to scrape the
VM metrics is assumed to be named `vm`.

Generate the validator.yaml file based on [validator.yaml.sample](validator.yaml.sample) and run the following command:
```console

python -m validator stress -s ./scripts/stressor.sh
```

## License

`validator` is distributed under the terms of the [Apache 2.0](http://www.apache.org/licenses/) license.
