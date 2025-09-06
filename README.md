# Gator
Gator is a blog aggregator. The name gator is ofcourse a little joke. Gator can be used to add blogs that you follow and combine them so you are able to view the latest posts of all those blogs in one place.

## Features
- **agg** aggregate over the blogs you follow
- **browse** find the newest posts
- **register** register an account
- **login** login to your account
- **addfeed** add a blog/feed so you can follow it
- **follow** follow an added feed
- **unfollow** unfollow a feed you are following. this can be undone by using follow
- **following** list the feeds you are following
- **feeds** lists all the feeds that you added
- **users** lists all the users on your machine
- **reset** if you want to you can fully reset everything


## Install
- First of check if you have go installed using
  ```bash
  go version
  ```
  If it shows a version move to the next step. If not install [go](https://go.dev/doc/install) and follow the instructions.

- Secondly download [postgress](https://www.postgresql.org/download/) if you had not already.

- Then use
```bash
go install https://github.com/Thijs-Desjardijn/gator
```
and everything is set for use.

## Usage  
You can view all the commands at feature, but here is a more in depth explanation. **You have to use:** *./gator "the command" "the arguments seperated by spaces"*                                                            
  
- **agg**  
Description: aggregate over the feeds that you added.  
Args: the time between the aggretations in this format: 1s, 1m, 1h etc.  
  
- **browse**  
Description: browse through your added feeds/blogs.  
Args: a limit of the amount of posts you would like to see wich defaults to 2.   
  
- **register**  
Description: register an account by using a name wich must be unique.  
Args: a name that can be anything aslong as it is unique.  
  
- **login**  
Description: login to your account.  
Args: the name of the user.  
  
- **addfeed**  
Description: add a blog/feed so you can follow it.  
Args: the name you want to give the feed and a url to the feed.  
  
- **follow**  
Description: follow an added feed.  
Args: the url of the feed.  
  
- **unfollow**  
Description: unfollow a feed you are following. this can be undone by using follow.  
Args: the url of the feed.  
  
- **following**  
Description: list the feeds you are following.  
Args: the name of the user.  
  
- **feeds**  
Description: lists all the feeds that you added.  
Args: no arguments required.  
  
- **users**  
Description: lists all the users on your machine.  
Args: no arguments required.  
  
- **reset**  
Description: if you want to you can fully reset everything. Note that this irreversable.  
Args: no arguments required.  


  
