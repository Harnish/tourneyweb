TourneyWeb is an OpenSource project for managing Tournaments.  Initially it is for baseball but should work just fine for other types of tournaments.

Check out the TODO if you want to help out.  

# Building
```
go build
```

# Creating a MySQL users/database
```
create database tourneyweb;
create user tourneyweb1@localhost IDENTIFIED BY 'password';
GRANT ALL PRIVILEGES ON tourneyweb.* to 'tourneyweb1'@'localhost';
```
use a different username and password for better security.

# Configuration file

Copy `tourneyweb.conf.example` to `tourneyweb.conf` and fill in your values. The config file is excluded from git.

```
cp tourneyweb.conf.example tourneyweb.conf
```

```yaml
---
port: 8989
debug: false
database: mysql://tourneyweb1:yourpassword@tcp(localhost:3306)/tourneyweb
adminpassword: yourpassword
disabledelete: false
bannerimagepath: dawgpoundlogo.jpg
```

