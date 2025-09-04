package main

import (
	"context"
	_ "crypto/aes"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Thijs-Desjardijn/gator/internal/database"
	"github.com/google/uuid"

	"github.com/Thijs-Desjardijn/gator/internal/config"
	_ "github.com/lib/pq"
)

type state struct {
	cfg *config.Config
	db  *database.Queries
}

type command struct {
	Name      string
	Arguments []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

const dbURL = "postgres://postgres:postgres@localhost:5432/gator"

var cmds commands

func registerCommands() error {
	err := cmds.register("users", handlerUsers)
	if err != nil {
		return err
	}
	err = cmds.register("login", handlerLogin)
	if err != nil {
		return err
	}
	err = cmds.register("register", handlerRegister)
	if err != nil {
		return err
	}
	err = cmds.register("reset", handlerReset)
	if err != nil {
		return err
	}
	err = cmds.register("feeds", handlerFeeds)
	if err != nil {
		return err
	}
	err = cmds.register("agg", handlerAgg)
	if err != nil {
		return err
	}
	err = cmds.register("browse", handlerBrowse)
	if err != nil {
		return err
	}
	//middleware required commands
	err = cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	if err != nil {
		return err
	}
	err = cmds.register("follow", middlewareLoggedIn(handlerFollow))
	if err != nil {
		return err
	}
	err = cmds.register("following", middlewareLoggedIn(handlerFollowing))
	if err != nil {
		return err
	}
	err = cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	if err != nil {
		return err
	}
	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return &RSSFeed{}, err
	}
	req.Header["User-Agent"] = []string{"gator"}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return &RSSFeed{}, err
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return &RSSFeed{}, err
	}
	var feed RSSFeed
	err = xml.Unmarshal(data, &feed)
	if err != nil {
		return &RSSFeed{}, err
	}
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	return &feed, nil
}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.handlers[cmd.Name]
	if !ok {
		return errors.New("command does not exist")
	}
	return handler(s, cmd)
}

func (c *commands) register(name string, f func(*state, command) error) error {
	c.handlers[name] = f
	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.Arguments) < 1 {
		return errors.New("expected argument in format '1s'")
	}
	timeBetweenReqs := cmd.Arguments[0]
	timeBetweenRequests, err := time.ParseDuration(timeBetweenReqs)
	if err != nil {
		return fmt.Errorf("error: %v\nexpected argument in format '1s'", err)
	}
	if timeBetweenRequests < (1 * time.Second) {
		fmt.Println("To not disturb other servers,\nthe time between requests has been defaulted to 1 second")
		timeBetweenRequests = 1 * time.Second
	}
	fmt.Printf("Collecting feeds every %v\n", timeBetweenReqs)
	ticker := time.NewTicker(timeBetweenRequests)
	for ; ; <-ticker.C {
		err = scrapeFeeds(s)
		if err != nil {
			fmt.Printf("error scraping this feed: %v\n", err)
		}
	}
}

func handlerBrowse(s *state, cmd command) error {
	var limit int
	if len(cmd.Arguments) < 1 {
		limit = 2
	} else {
		number, err := strconv.Atoi(cmd.Arguments[0])
		if err != nil {
			fmt.Printf("Invalid number: %v\nexpected a number like: 3\ndefaulted to a limit of 2\n", cmd.Arguments[0])
			limit = 2
		} else {
			limit = number
		}
	}
	user, err := s.db.GetUser(context.Background(), sql.NullString{String: s.cfg.CurrentUserName, Valid: true})
	if err != nil {
		return err
	}
	args := database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(limit),
	}
	posts, err := s.db.GetPostsForUser(context.Background(), args)
	if err != nil {
		return err
	}
	if len(posts) < 1 {
		return errors.New("no posts were found")
	}
	for i := 0; i < limit; i++ {
		feed, err := s.db.GetFeedForID(context.Background(), posts[i].FeedID)
		if err != nil {
			fmt.Println(err)
			continue
		}
		err = s.db.MarkFeedFetched(context.Background(), feed.ID)
		if err != nil {
			fmt.Println(err)
			continue
		}
		RSSFeed, err := fetchFeed(context.Background(), feed.Url)
		if err != nil {
			fmt.Println(err)
			continue
		}
		for _, item := range RSSFeed.Channel.Item {
			fmt.Printf("Title: %v\nUrl: %v\n\n", item.Title, item.Link)
			i += 1
			if i >= limit {
				break
			}
		}
	}
	return nil
}

func scrapeFeeds(s *state) error {
	feed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return err
	}
	err = s.db.MarkFeedFetched(context.Background(), feed.ID)
	if err != nil {
		return err
	}
	RSSFeed, err := fetchFeed(context.Background(), feed.Url)
	if err != nil {
		fmt.Printf("error: %v\ninside feed: %v\nurl: %v", err, feed.Name, feed.Url)
		return nil
	}
	timeParcing := []string{"Mon, 02 Jan 2006 15:04:05 -0700", "Mon, 02 Jan 2006 15:04:05 MST",
		"02 Jan 06 15:04 MST", "02 Jan 06 15:04 -0700"}
	for _, item := range RSSFeed.Channel.Item {
		fmt.Printf("Title: %v\npublication date: %v\n", item.Title, item.PubDate)
		var publicationDate time.Time
		for _, format := range timeParcing {
			publicationDate, err = time.Parse(format, item.PubDate)
			if err != nil {
				continue
			} else {
				break
			}
		}
		if err != nil {
			var date time.Time
			fmt.Printf("title: %v url: %v\nTime formatting did not succeed. Defaulted to the zero value\n", item.Title, item.Link)
			publicationDate = date
		}

		args := database.CreatePostParams{
			CreatedAt:   sql.NullTime{Time: time.Now(), Valid: true},
			UpdatedAt:   sql.NullTime{Time: time.Now(), Valid: true},
			Title:       sql.NullString{String: item.Title, Valid: true},
			Url:         item.Link,
			Description: sql.NullString{String: item.Description, Valid: true},
			PublishedAt: sql.NullTime{Time: publicationDate, Valid: true},
			FeedID:      feed.ID,
		}
		_, err = s.db.CreatePost(context.Background(), args)
		if err != nil {
			if strings.Contains(err.Error(), "unique constraint") && strings.Contains(err.Error(), "posts_url_key") {
				continue
			}
			return err
		}
	}
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.Arguments) < 1 {
		return errors.New("expected argument: 'feed url'")
	}
	args := database.DeleteFeedFollowParams{
		UserID: user.ID,
		Url:    cmd.Arguments[0],
	}
	err := s.db.DeleteFeedFollow(context.Background(), args)
	if err != nil {
		return err
	}
	return nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.Arguments) < 2 {
		return errors.New("expected arguments: 'name' 'url'")
	}
	args := database.CreateFeedParams{
		Name:   cmd.Arguments[0],
		Url:    cmd.Arguments[1],
		UserID: user.ID,
	}
	_, err := s.db.CreateFeed(context.Background(), args)
	if err != nil {
		return err
	}
	cmd.Arguments[0] = cmd.Arguments[1]
	err = handlerFollow(s, cmd, user)
	if err != nil {
		return err
	}
	return nil
}

func handlerFollowing(s *state, _ command, user database.User) error {
	feeds, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}
	fmt.Println("Current feeds you are following:")
	for _, feed := range feeds {
		fmt.Printf("Title: %v\n", feed.FeedName)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.Arguments) < 1 {
		return errors.New("expected argument: 'url'")
	}
	url := cmd.Arguments[0]
	feedId, err := s.db.GetFeedId(context.Background(), url)
	if err != nil {
		return err
	}
	args := database.CreateFeedFollowParams{
		CreatedAt: sql.NullTime{Time: time.Now(), Valid: true},
		UpdatedAt: sql.NullTime{Time: time.Now(), Valid: true},
		UserID:    user.ID,
		FeedID:    feedId,
	}
	s.db.CreateFeedFollow(context.Background(), args)
	return nil
}

func handlerFeeds(s *state, _ command) error {
	allFeeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return err
	}
	for _, feed := range allFeeds {
		userName, err := s.db.GetUserName(context.Background(), feed.UserID)
		if err != nil {
			return err
		}
		fmt.Printf("Title: %v\nUrl: %v\nAuthor: %v\n\n", feed.Name, feed.Url, userName.String)
	}
	return nil
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.Arguments) < 1 {
		return errors.New("error: no arguments given")
	}
	_, err := s.db.GetUser(context.Background(), sql.NullString{String: cmd.Arguments[0], Valid: true})
	if err != nil {
		return err
	}
	err = s.cfg.SetUser(cmd.Arguments[0])
	if err != nil {
		return err
	}
	fmt.Printf("username has been set to: %s\n", cmd.Arguments[0])
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.Arguments) < 1 {
		return errors.New("error: No name was given")
	}
	var args database.CreateUserParams
	args.CreatedAt = time.Now()
	args.UpdatedAt = time.Now()
	args.ID = uuid.New()
	args.Name = sql.NullString{String: cmd.Arguments[0], Valid: true}
	_, err := s.db.GetUser(context.Background(), args.Name)
	if err == nil {
		os.Exit(1)
	}
	user, err := s.db.CreateUser(context.Background(), args)
	if err != nil {
		return err
	}
	s.cfg.CurrentUserName = cmd.Arguments[0]
	fmt.Printf("userdata: %v", user)
	err = s.cfg.SetUser(s.cfg.CurrentUserName)
	if err != nil {
		return err
	}
	return nil
}

func handlerReset(s *state, _ command) error {
	err := s.db.RemoveUsers(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func handlerUsers(s *state, _ command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}
	for _, user := range users {
		userName := fmt.Sprintf("%v", user.String)
		fmt.Printf("* %v", userName)
		if userName == s.cfg.CurrentUserName {
			fmt.Printf(" (current)")
		}
		fmt.Printf("\n")
	}
	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, c command) error {
		user, err := s.db.GetUser(context.Background(), sql.NullString{String: s.cfg.CurrentUserName, Valid: true})
		if err != nil {
			return err
		}
		return handler(s, c, user)
	}
}

func main() {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	if len(os.Args) < 2 {
		log.Fatal("No command or arguments given")
	}
	c, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}
	programState := &state{
		cfg: &c,
		db:  dbQueries,
	}
	cmds = commands{
		handlers: make(map[string]func(*state, command) error),
	}
	err = registerCommands()
	if err != nil {
		log.Fatal(err)
	}
	var Command command
	Command.Arguments = os.Args[2:]
	Command.Name = os.Args[1]
	err = cmds.run(programState, Command)
	if err != nil {
		log.Fatal(err)
	}
}
