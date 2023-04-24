# TBSTB - Ticket-based Support Telegram Bot

A Telegram bot for support via ticket-based interactions.

## Installation 

~~~
git clone https://github.com/Charibdys/tbstb.git
cd tbstb
go build
~~~

## Purpose of the program:

The goal of this bot is to give administrative users, whether admins of group chats, channels, lounge bots, or similar, a consolidated and easy-to-use tool to address
the issues and inquiries of their user base.

TBSTB can be used in private chats, or in group chats.

TBSTB can allow multiple admins/support representatives to address tickets .

TBSTB can allow admins/support representatives to remain anonymous, use a pseudonym, or their Telegram name/username.

TBSTB will not have a config file; all attributes of TBSTB will be stored in the database; keys will be passed as environment variables.

Users can open a ticket; this ticket saves the message history and relays it to the admins.

Admins can assign tickets to support representatives.

One or more admins/support representatives can reserve a ticket and close it.

Admins/support representatives can access open tickets within telegram via a given user interface.
## What TBSTB is not:

TBSTB is not a group chat administration bot (such as CalsiBot, Rose, etc).

TBSTB is not a single-instance bot; if TBSTB is to be used, a user must make a bot with BotFather and host their own instance of TBSTB.

## Roadmap:

- [] 0.1 - Connect to Telegram, receive updates; Database CRUD operations
- [] 0.2 - Ticket generation
- [] 0.3 - Resolve tickets
- [] 0.X - Compatibility with group chats, ticket user interface, user-defined names/anonymity, promoting users, content management, statistics

## Contributing

Open tasks and current planned features can be found in this repo's project, [TBSTB Development](https://github.com/users/Charibdys/projects/3)

If you would like to make a contribution, follow these steps:

1. Fork it (<https://github.com/Charibdys/tbstb/fork>)
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create a new Pull Request

Ensure that your code is documented and follows the [Effective Go coding style](https://go.dev/doc/effective_go).

## Contributors

- [Charybdis](https://gitlab.com/Charibdys) - creator and maintainer

