package main

import (
	"github.com/samalba/dockerclient"
	"log"
	"os"

	//"encoding/binary"
	"flag"
	"fmt"

	"github.com/streadway/amqp"
	"strings"
	"time"

	"github.com/banthar/Go-SDL/sdl"
	"github.com/banthar/Go-SDL/ttf"
)
import l4g "code.google.com/p/log4go"

var introw int16 = 0
var fontsize int16 = 16
var winTitle string = "Go-SDL2 Render"
var previousStr string = ""
var winWidth, winHeight int = 640, 480
var screen = sdl.SetVideoMode(winWidth, winHeight, 0, sdl.RESIZABLE)
var bgcolor uint32 = 0x03A9F4

var (
	uri          = flag.String("uri", "amqp://guest:guest@172.17.42.1:5672/", "AMQP URI")
	exchange     = flag.String("exchange", "e6", "Durable, non-auto-deleted AMQP exchange name")
	exchangeType = flag.String("exchange-type", "fanout", "Exchange type - direct|fanout|topic|x-custom")
	queue        = flag.String("queue", "q6", "Ephemeral AMQP queue name")
	bindingKey   = flag.String("key", "test-key", "AMQP binding key")
	consumerTag  = flag.String("consumer-tag", "simple-consumer", "AMQP consumer tag (should not be blank)")
	lifetime     = flag.Duration("lifetime", 0*time.Second, "lifetime of process before shutdown (0s=infinite)")
)

func init() {
	flag.Parse()
}

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
}

func NewConsumer(amqpURI, exchange, exchangeType, queueName, key, ctag string) (*Consumer, error) {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     ctag,
		done:    make(chan error),
	}

	var err error

	//l4g.Info("dialing %q", amqpURI)
	var RS string = fmt.Sprintf("dialing \n%q", amqpURI)
	l4g.Info(RS)
	writeSDLstr(RS)

	c.conn, err = amqp.Dial(amqpURI)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		fmt.Printf("closing: %s", <-c.conn.NotifyClose(make(chan *amqp.Error)))
	}()

	//l4g.Info("got Connection, getting Channel")
	RS = fmt.Sprintf("got Connection, getting Channel")
	l4g.Info(RS)
	writeSDLstr(RS)

	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("Channel: %s", err)
	}

	//l4g.Info("got Channel, declaring Exchange (%q)", exchange)
	RS = fmt.Sprintf("got Channel, declaring Exchange (%q)", exchange)
	l4g.Info(RS)
	writeSDLstr(RS)

	if err = c.channel.ExchangeDeclare(
		exchange,     // name of the exchange
		exchangeType, // type
		false,        // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		return nil, fmt.Errorf("Exchange Declare: %s", err)
	}

	//l4g.Info("declared Exchange, declaring Queue %q", queueName)
	RS = fmt.Sprintf("declared Exchange, declaring Queue %q", queueName)
	l4g.Info(RS)
	writeSDLstr(RS)

	queue, err := c.channel.QueueDeclare(
		queueName, // name of the queue
		false,     // durable
		false,     // delete when usused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Declare: %s", err)
	}

	//l4g.Info("declared Queue (%q %d messages, %d consumers), binding to Exchange (key %q)",
	//	queue.Name, queue.Messages, queue.Consumers, key)
	RS = fmt.Sprintf("declared Queue (%q %d messages, %d consumers), binding to Exchange (key %q)",
		queue.Name, queue.Messages, queue.Consumers, key)
	l4g.Info(RS)
	writeSDLstr(RS)

	if err = c.channel.QueueBind(
		queue.Name, // name of the queue
		key,        // bindingKey
		exchange,   // sourceExchange
		false,      // noWait
		nil,        // arguments
	); err != nil {
		return nil, fmt.Errorf("Queue Bind: %s", err)
	}

	//l4g.Info("Queue bound to Exchange, starting Consume (consumer tag %q)", c.tag)
	RS = fmt.Sprintf("Queue bound to Exchange, starting Consume (consumer tag %q)", c.tag)
	l4g.Info(RS)
	writeSDLstr(RS)

	deliveries, err := c.channel.Consume(
		queue.Name, // name
		c.tag,      // consumerTag,
		false,      // noAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Consume: %s", err)
	}

	go handle(deliveries, c.done)

	return c, nil
}

func (c *Consumer) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Consumer cancel failed: %s", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer l4g.Info("AMQP shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func handle(deliveries <-chan amqp.Delivery, done chan error) {

	for d := range deliveries {

		/*l4g.Info(
			"got %dB delivery: [%v] %q \n",
			len(d.Body),
			d.DeliveryTag,
			d.Body,
		)*/
		var a = make([]byte, len(d.Body))
		copy(a[:], d.Body)
		packetsize := a[1]
		var len = a[3]
		var ii byte = 4

		s1 := string(a[ii:(ii + len)])
		ii = ii + len + byte(1)

		len = a[ii] //byte(12)
		ii = ii + byte(1)
		s2 := string(a[ii:(ii + len)])
		ii = ii + len + byte(1)

		len = a[ii] //byte(12)
		ii = ii + byte(1)
		s3 := string(a[ii:(ii + len)])
		ii = ii + len + byte(1)

		len = a[ii] //byte(12)
		ii = ii + byte(1)
		s4 := string(a[ii:(ii + len)])

		//fmt.Printf("AMQP : packetsize=%d\n exchange=%v\n listsensor.ini=%v\n displayport=%v\n VMIP=%v\n ", packetsize, s1, s2, s3, s4)
		var RS string = fmt.Sprintf("AMQP : packetsize=%d\n exchange=%v\n listsensor.ini=%v\n displayport=%v\n VMIP=%v\n ", packetsize, s1, s2, s3, s4)
		l4g.Info(RS)
		writeSDLstr(RS)

		docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
		if err != nil {
			log.Fatal(err)
		}

		if strings.EqualFold(s1, "create") {
			startDocker(docker, s2, s3, s4)
		} else if strings.EqualFold(s1, "remove") {
			var dockerName string = "docker" + s3
			supprDocker(docker, dockerName)
		}

		d.Ack(false)
	}

	l4g.Info("handle: deliveries channel closed")
	done <- nil
}

// Callback used to listen to Docker's events
func eventCallback(event *dockerclient.Event, ec chan error, args ...interface{}) {
	l4g.Info("Received event: %#v\n", *event)
}

func supprDocker(docker *dockerclient.DockerClient, dockerName string) {

	var searchDockerName string = "{\"name\":[\"" + dockerName + "\"]}"
	//l4g.Info("%v\n", searchDockerName)
	// Get only running containers
	containers, err := docker.ListContainers(true, true, searchDockerName)
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range containers {
		l4g.Info(c.Id, c.Names)

		if err := docker.RemoveContainer(c.Id, true, false); err != nil {
			//l4g.Info("cannot stop container: %s", err)
			var RS string = fmt.Sprintf("cannot stop container: %s", err)
			l4g.Info(RS)
			writeSDLstr(RS)

		}
	}
}

func startDocker(docker *dockerclient.DockerClient, s2 string, s3 string, s4 string) {

	var dirpath string = "/tmp/shareDocker" + s3
	os.RemoveAll(dirpath)
	os.Mkdir(dirpath, 0777)
	var filepath string = dirpath + "/config.ini"

	//l4g.Info("startDocker -> %v", filepath)
	var RS string = fmt.Sprintf("startDocker -> %v", filepath)
	l4g.Info(RS)
	writeSDLstr(RS)

	f, _ := os.Create(filepath)
	f.WriteString("ip_broker=rabbitmq\n")
	f.Sync()
	f.WriteString("\n")
	f.Sync()
	f.WriteString(s2)
	f.Sync()
	f.WriteString("\n")
	defer f.Close()

	//l4g.Info("Init the client")
	RS = fmt.Sprintf("Init the client")
	l4g.Info(RS)
	writeSDLstr(RS)
	l4g.Info(os.Getenv("DOCKER_HOST"))

	//l4g.Info("DOCKER_HOST")
	RS = fmt.Sprintf("DOCKER_HOST")
	l4g.Info(RS)
	writeSDLstr(RS)

	var dockerName string = "docker" + s3

	supprDocker(docker, dockerName)

	//l4g.Info(" Create a container")
	RS = fmt.Sprintf(" Create a container")
	l4g.Info(RS)
	writeSDLstr(RS)
	containerConfig := &dockerclient.ContainerConfig{
		Image: "hwgrepo:hwg",
		User:  "developer",
		//		Env: []string{"PATH=\/home\/developer\/sdl_sensor_broker\/"},
		Cmd:          []string{"sh"},
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
		Tty:          true,
		OpenStdin:    true,
		StdinOnce:    false,
	}
	containerId, err := docker.CreateContainer(containerConfig, dockerName)

	if err != nil {
		log.Fatal(err)
	}
	var port_tcp string = s3 + "/tcp"
	portBinding := map[string][]dockerclient.PortBinding{port_tcp: {{"", s3}}}

	var sharedDocker string = dirpath + ":/home/developer/sdl_sensor_broker/log"

	l4g.Info(" Start the container")
	hostConfig := &dockerclient.HostConfig{
		Binds:        []string{sharedDocker, "/tmp/.X11-unix:/tmp/.X11-unix:rw"},
		Privileged:   true,
		PortBindings: portBinding,
		Links:        []string{"rabbitmq:rabbitmq"},
	}
	err = docker.StartContainer(containerId, hostConfig)
	if err != nil {
		log.Fatal(err)
	}

	execConfig := &dockerclient.ExecConfig{
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
		Tty:          true,
		Cmd:          []string{"sh", "runNoVNCplayer.sh", s4, s3},
		Container:    containerId,
		Detach:       true}

	_, err = docker.Exec(execConfig)

	if err != nil {
		log.Fatal(err)
		l4g.Info(" error exec container")
		RS = fmt.Sprintf(" error exec container")
		l4g.Info(RS)
		writeSDLstr(RS)
	}

}

func main() {
	screen.FillRect(nil, bgcolor)
	go ui_main()

	///////////////////////////////////////////////////////////////
	///////////////////////////////////////////////////////////////
	///////////////////////////////////////////////////////////////

	flw := l4g.NewFileLogWriter("/tmp/golog.log", false)
	flw.SetFormat("[%D %T] [%L] GOMANAGER : %M ")
	flw.SetRotate(false)
	flw.SetRotateSize(0)
	flw.SetRotateLines(0)
	flw.SetRotateDaily(false)
	l4g.AddFilter("file", l4g.FINE, flw)
	l4g.Info(" --- START GOMANAGER ---")
	writeSDLstr(" --- START GOMANAGER ---")

	c, err := NewConsumer(*uri, *exchange, *exchangeType, *queue, *bindingKey, *consumerTag)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if *lifetime > 0 {
		l4g.Info("running for %s", *lifetime)
		time.Sleep(*lifetime)
	} else {
		l4g.Info("running forever")
		writeSDLstr("running forever")
		select {}
	}

	l4g.Info("shutting down")

	if err := c.Shutdown(); err != nil {
		log.Fatalf("error during shutdown: %s", err)
	}

}

func ui_main() {

	if sdl.Init(sdl.INIT_VIDEO) != 0 {
		panic(sdl.GetError())
	}

	if screen == nil {
		panic(sdl.GetError())
	}

	sdl.EnableUNICODE(1)

	sdl.WM_SetCaption("Go-SDL SDL Test", "")

	running := true

	for running {
		for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
			switch e := ev.(type) {
			case *sdl.QuitEvent:
				running = false
				break

			case *sdl.ResizeEvent:
				println("resize screen ", e.W, e.H)

				screen = sdl.SetVideoMode(int(e.W), int(e.H), 32, sdl.RESIZABLE)

				if screen == nil {
					panic(sdl.GetError())
				}
			}
		}

	}

	ttf.Quit()
	sdl.Quit()
}

func writeSDLstr(newStr string) {

	if ttf.Init() != 0 {
		panic(sdl.GetError())
	}
	var font = ttf.OpenFont("FontinSans.otf", int(fontsize))

	blue := sdl.Color{185, 230, 255, 0}
	white := sdl.Color{255, 255, 255, 0}
	text0 := ttf.RenderText_Blended(font, previousStr, blue)
	font.SetStyle(ttf.STYLE_UNDERLINE)
	text1 := ttf.RenderText_Blended(font, newStr, white)

	introw = introw - fontsize - fontsize/2
	screen.FillRect(&sdl.Rect{3, introw, uint16(winWidth - 3), uint16(fontsize + fontsize/2)}, bgcolor)
	screen.Blit(&sdl.Rect{3, introw, 0, 0}, text0, nil)
	introw = introw + fontsize + fontsize/2
	screen.Blit(&sdl.Rect{3, introw, 0, 0}, text1, nil)
	introw = introw + fontsize + fontsize/2
	if introw > (int16(winHeight) - fontsize) {
		introw = 0
	}
	screen.FillRect(&sdl.Rect{3, introw, uint16(winWidth - 3), uint16(fontsize + fontsize/2)}, bgcolor)

	screen.Flip()
	previousStr = newStr

}

