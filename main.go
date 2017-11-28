package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/bearyinnovative/bearychat-go"
	"github.com/go-redis/redis"
)

type State struct {
	Me            *bearychat.User
	RedisClient   *redis.Client
	RtmClient     *bearychat.RTMClient
	RtmLoop       bearychat.RTMLoop
	FatherChannel *bearychat.P2P
	ATuring       *TuringClient
}

func info(uid, vChannelId string, state *State) {
	text := fmt.Sprintf("我的名字是 %s, 我的父亲是唐晓敏，请多多关照", state.Me.Name)
	sendMsg(uid, vChannelId, text, state)
}

var rtmToken string
var authorUID string

func init() {
	flag.StringVar(&rtmToken, "rtmToken", "", "BearyChat RTM token")
	flag.StringVar(&authorUID, "authorUID", "", "My father's ID")
}

func main() {
	flag.Parse()
	redisClient := createRedisClient()
	turingClient := NewTuringClient("9d213379fd6b41e3b8dc73aeb193fae8")
	rtmClient, err := bearychat.NewRTMClient(
		rtmToken,
		bearychat.WithRTMAPIBase("https://api.bearychat.com"),
	)
	checkErr(err)

	theHubot, wsHost, err := rtmClient.Start()
	checkErr(err)

	rtmLoop, err := bearychat.NewRTMLoop(wsHost)
	checkErr(err)

	checkErr(rtmLoop.Start())
	defer rtmLoop.Stop()

	go rtmLoop.Keepalive(time.NewTicker(2 * time.Second))

	errC := rtmLoop.ErrC()
	messageC, err := rtmLoop.ReadC()
	checkErr(err)

	// user, _ := rtmClient.User.Info(authorUID)
	p2p, err := rtmClient.P2P.Create(authorUID)
	checkErr(err)

	state := &State{
		Me:            theHubot,
		RedisClient:   redisClient,
		RtmClient:     rtmClient,
		RtmLoop:       rtmLoop,
		FatherChannel: p2p,
		ATuring:       turingClient,
	}

	for {
		select {
		case err := <-errC:
			checkErr(err)
			return
		case message := <-messageC:
			if message.IsFromUser(*theHubot) {
				continue
			}
			if !message.IsChatMessage() {
				continue
			}
			if message.IsP2P() {
				processP2PMessage(message, state)
			} else {
				processChannelMessage(message, state)
			}
		}
	}
}

func processChannelMessage(message bearychat.RTMMessage, state *State) {
	if message["uid"] != nil {
		uid := message["uid"].(string)
		vChannelId := message["vchannel_id"].(string)
		text := message["text"].(string)
		if mentioned, _ := message.ParseMentionUID(authorUID); mentioned {
			userA, _ := state.RtmClient.User.Info(uid)
			content := fmt.Sprintf("[%s：%s]", userA.Name, text)
			sendToBaBa(content, state)
		} else if mentioned, _ := message.ParseMentionUser(*state.Me); mentioned {
			if response, ok := state.ATuring.Do(text, uid); ok {
				sendMsg(uid, vChannelId, response, state)
			}
		} else {
			if shouldTellBigBro(text) {
				userA, _ := state.RtmClient.User.Info(uid)
				content := fmt.Sprintf("[%s：%s]", userA.Name, text)
				sendToBaBa(content, state)
			}
		}
	}
}

func shouldTellBigBro(text string) bool {
	return strings.Contains(text, "后端")
}

func getTodoTaskHash(uid string) string {
	return fmt.Sprintf("task:%s:todo", uid)
}

func getMonitorHash(uid string) string {
	return fmt.Sprintf("monitor:%s", uid)
}

func createRedisClient() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	_, err := client.Ping().Result()
	if err != nil {
		log.Fatal("create redis client failed")
		return nil
	}
	return client
}

func processP2PMessage(message bearychat.RTMMessage, state *State) {
	textLower := strings.ToLower(message["text"].(string))
	vChannelId := message["vchannel_id"].(string)
	uid := message["uid"].(string)
	if strings.Contains(textLower, "todo") {
		addTask(message, state)
		sendMsg(uid, vChannelId, "添加成功，主人现在任务有: ", state)
		time.Sleep(100 * time.Millisecond)
		showTasks(uid, vChannelId, state)
	} else if strings.Contains(textLower, "show") {
		showTasks(uid, vChannelId, state)
	} else if strings.Contains(textLower, "done") {
		clearTasks(message, state)
		sendMsg(uid, vChannelId, "任务清理成功，主人现在任务有: ", state)
		showTasks(uid, vChannelId, state)
	} else if strings.Contains(textLower, "monitor") {
		addMonitor(message, state)
		showMonitor(uid, vChannelId, state)
	} else {
		if response, ok := state.ATuring.Do(textLower, uid); ok {
			sendMsg(uid, vChannelId, response, state)
		} else {
			info(uid, vChannelId, state)
		}
	}
}

func addMonitor(message bearychat.RTMMessage, state *State) {
	uid := message["uid"].(string)
	text := message["text"].(string)
	monitorHash := getMonitorHash(uid)
	text = strings.TrimSpace(text)
	text = strings.TrimLeft(text, "monitor")
	text = strings.TrimSpace(text)
	state.RedisClient.SAdd(monitorHash, text)
}

func showMonitor(uid, vChannelId string, state *State) {
	monitors, _ := state.RedisClient.SMembers(getMonitorHash(uid)).Result()
	if monitors != nil {
		var text string
		if len(monitors) == 0 {
			text = "No monitor worker"
		} else {
			text = "Monitoring: \n"
			for i, _ := range monitors {
				text = text + monitors[i] + "\t"
			}
		}
		sendMsg(uid, vChannelId, text, state)
	}
}

func addTask(message bearychat.RTMMessage, state *State) {
	vChannelId := message["vchannel_id"].(string)
	uid := message["uid"].(string)
	todoHash := getTodoTaskHash(uid)
	if message["refer_key"] != nil {
		referKey := message["refer_key"].(string)
		log.Printf("with refer: %s", referKey)
		messageP, _ := state.RtmClient.Message.Info(vChannelId, referKey)
		message = *messageP
	}
	text := message["text"].(string)
	text = strings.TrimSpace(text)
	text = strings.TrimLeft(text, "todo")
	text = strings.TrimSpace(text)
	log.Printf("task [%s] added\n", text)
	created := message["created_ts"].(float64)
	ele := redis.Z{Score: created, Member: text}
	state.RedisClient.ZAdd(todoHash, ele)
}

func clearTasks(message bearychat.RTMMessage, state *State) {
	text := strings.ToLower(message["text"].(string))
	uid := message["uid"].(string)
	todoKey := getTodoTaskHash(uid)
	doneStr := strings.TrimSpace(strings.TrimLeft(text, "done"))
	dones := strings.Split(doneStr, " ")
	if dones != nil && len(dones) > 0 {
		for _, done := range dones {
			doneIndex, _ := strconv.Atoi(done)
			log.Printf("task %d done", doneIndex)
			doneIndex--
			state.RedisClient.ZRemRangeByRank(todoKey, int64(doneIndex), int64(doneIndex))
		}
	}
}

func showTasks(uid, vChannelId string, state *State) {
	todos, _ := state.RedisClient.ZRange(getTodoTaskHash(uid), 0, -1).Result()
	if todos != nil {
		var text string
		if len(todos) == 0 {
			text = "工作都完成了，好棒!"
		} else {
			for i, _ := range todos {
				todos[i] = strconv.Itoa(i+1) + ". " + todos[i]
			}
			text = strings.Join(todos, "\n")
		}
		sendMsg(uid, vChannelId, text, state)
	}
}

func sendToBaBa(text string, state *State) {
	sendMsg(authorUID, state.FatherChannel.VChannelId, text, state)
}

func sendMsg(uid, vChannelId, text string, state *State) {
	m := bearychat.RTMMessage{
		"type":        bearychat.RTMMessageTypeP2PMessage,
		"vchannel_id": vChannelId,
		"to_uid":      uid,
		"call_id":     rand.Int(),
		"refer_key":   nil,
		"text":        text,
	}
	state.RtmLoop.Send(m)
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
