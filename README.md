TourneyWeb is an OpenSource project for managing Tournaments.  Initially it is for baseball but should work just fine for other types of tournaments.

Check out the TODO if you want to help out.  

=== Building
```
go build
```

=== Creating a MySQL users/database
```
create database tourneyweb;
create user tourneyweb1@localhost IDENTIFIED BY 'password';
GRANT ALL PRIVILEGES ON tourneyweb.* to 'tourneyweb1'@'localhost';
```
use a different username and password for better security.

=== Configuration file
```
---
port: 8989
debug: true
database: mysql://tourneyweb1:password@tcp(localhost:3306)/tourneyweb
adminpassword: adminpassword1
```

