# tradingServer
A simple restAPI trading server implemented in go. 

It offers... 
* user authentication and personal accounts, 
* buy and sell items and 
* simulates price fluctuations. 

Backed with an sqlite3 database.

## How to setup

Compile and run the binary tradingServer. It will create the database upon first run.

To add new users, run ./tradingServer adduser <login> <password> [<email>]
This will create the user account, store the hashed password and grant the user a starting balance of 100 credits.

## How to use
Start the tradingServer and visit the URL http://localhost:8002/ with your browser. It will show short instructions for each possible API endpoint.
