# WiiLink WFC
WiiLink Wi-Fi Connection is an open source server replacement for the late Nintendo Wi-Fi Connection, supporting both Nintendo DS and Wii games. This repository contains the server-side source code.

## Setup
You will need:
- MariaDB

1. Create a MariaDB database. Note the database name, username, and password.
2. Use the `schema.sql` found in the root of this repo and import it into your MariaDB database.
3. Copy `config-example.xml` to `config.xml` and insert all the correct data.
4. Run `go build`. The resulting executable `wwfc` is the executable of the server.
