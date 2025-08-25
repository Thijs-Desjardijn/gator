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
	err = cmds.register("agg", handlerAgg)
	if err != nil {
		return err
	}
	err = cmds.register("addfeed", handlerAddFeed)
	if err != nil {
		return err
	}
	err = cmds.register("feeds", handlerFeeds)
	if err != nil {
		return err
	}
	err = cmds.register("follow", handlerFollow)
	if err != nil {
		return err
	}
	err = cmds.register("following", handlerFollowing)
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

func handlerAgg(_ *state, _ command) error {
	fetchedRss, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return err
	}
	fmt.Println(fetchedRss)
	return nil
}

func handlerAddFeed(s *state, cmd command) error {
	if len(cmd.Arguments) < 2 {
		return errors.New("expected arguments: 'name' 'url'")
	}
	user, err := s.db.GetUser(context.Background(), sql.NullString{String: s.cfg.CurrentUserName, Valid: true})
	if err != nil {
		return err
	}
	args := database.CreateFeedParams{
		Name:   cmd.Arguments[0],
		Url:    cmd.Arguments[1],
		UserID: user.ID,
	}
	_, err = s.db.CreateFeed(context.Background(), args)
	if err != nil {
		return err
	}
	cmd.Arguments[0] = cmd.Arguments[1]
	err = handlerFollow(s, cmd)
	if err != nil {
		return err
	}
	return nil
}

func handlerFollowing(s *state, _ command) error {
	user, err := s.db.GetUser(context.Background(), sql.NullString{String: s.cfg.CurrentUserName, Valid: true})
	if err != nil {
		return err
	}
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

func handlerFollow(s *state, cmd command) error {
	if len(cmd.Arguments) < 1 {
		return errors.New("expected argument: 'url'")
	}
	url := cmd.Arguments[0]
	feedId, err := s.db.GetFeedId(context.Background(), url)
	if err != nil {
		return err
	}
	user, err := s.db.GetUser(context.Background(), sql.NullString{String: s.cfg.CurrentUserName, Valid: true})
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

const dbURL = "postgres://postgres:postgres@localhost:5432/gator"

var cmds commands

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
		cfg: &c, // where c is your config
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
