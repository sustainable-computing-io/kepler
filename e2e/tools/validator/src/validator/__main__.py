# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0
import sys

if __name__ == "__main__":
    from validator.cli import validator

    sys.exit(validator())
