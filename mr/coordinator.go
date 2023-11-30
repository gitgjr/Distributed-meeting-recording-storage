package mr

import (
	"fmt"
	"log"
	"main/utils"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type Coordinator struct {
	Workers  map[string]*Worker //[workerID]*Worker
	NWorkers int
	Bucket
	NMapTask Task
	allTask  Task //all tasks
	// TaskChannel chan Task
	mutex sync.Mutex
}

//TODO: Message service and P2P service

type Bucket map[int]Task //WorkerID:Task

func NewCoordinator() *Coordinator {
	c := Coordinator{}
	c.Workers = make(map[string]*Worker)
	c.allTask = make(Task)
	return &c
}

// Run Boot Http server
func (c *Coordinator) Run() {
	http.ListenAndServe(":8080", nil)
}

func (c *Coordinator) Router() {
	http.HandleFunc("/", c.DefaultHandler)
	http.HandleFunc("/register", c.RegisterHandler)
	http.HandleFunc("/update", c.UpdateHandler)
	http.HandleFunc("/callReduce", c.callTransmitHandler)
}

func (c *Coordinator) PrintWorkers() {
	for _, worker := range c.Workers {
		utils.PrintStruct(worker)
	}
}

// ScanAllTask:Scan all registered creator and add all the task they hold into allTask
func (c *Coordinator) ScanAllTask() {
	for _, w := range c.Workers {
		err := MergeTasks(c.allTask, w.TaskList)
		if err != nil {
			panic(err)
		}
	}

}

func (c *Coordinator) returnOnlineWorker() ([]string, []*Worker) {
	onlineIDList := []string{}
	onlineWorkerList := []*Worker{}
	for id, w := range c.Workers {
		if w.State == "online" {
			onlineIDList = append(onlineIDList, id)
			onlineWorkerList = append(onlineWorkerList, w)
		}
	}
	return onlineIDList, onlineWorkerList
}

// AssignWork : Some method to assign task:1.if worker get this task assgin
// 2.Equally distributed according to the number of workers
// 3.Assign based on worker connections on the basis of 2
func (c *Coordinator) AssignReduceTask() TransmitTask {
	_, onlineWorkerList := c.returnOnlineWorker() //[WorkerID]
	newTransmitTaskSet := c.assginTaskM1(onlineWorkerList)
	return newTransmitTaskSet
}

func (c *Coordinator) assginTaskM1(onlineList []*Worker) ReduecTaskSet {

}

// CheckWorkers: check and update the state of workers
func (c *Coordinator) CheckWorkers() {
	var wg sync.WaitGroup
	for workerID := range c.Workers {
		wg.Add(1)
		go func(wid string) {
			defer wg.Done()
			err := c.sendCheckAndUpdate(wid)
			fmt.Println(err)

		}(workerID)
	}
	wg.Wait()
	// c.PrintWorkers()
}

func (c *Coordinator) sendCheckAndUpdate(workerID string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	targetWorker := c.Workers[workerID]
	res, err := http.Get(utils.SpliceUrl(targetWorker.Addr, targetWorker.Port, "checkState"))
	if err != nil {
		c.Workers[workerID].State = "offline"
		return err
	}
	if res.StatusCode != http.StatusOK {
		c.Workers[workerID].State = "offline"
		return nil
	}
	c.Workers[workerID].State = "online"
	c.Workers[workerID].LastOnlie = time.Now()
	return nil

}

// transmit: give a worker a command to transmit receivers:WorkerID of receivers,
// transmitTaskID:TaskID of tasks to be transmitted
func (c *Coordinator) transmit(sender *Worker, tTask TransmitTask) {
	for _, taskIDList := range tTask {
		for _, taskID := range taskIDList {
			_, ok := sender.TaskList[taskID]
			if ok == false {
				panic("task not found" + taskID)
			}
		}

	}
	res, err := SendPostRequest(tTask, utils.SpliceUrl(sender.Addr, sender.Port, "transmit"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res) //TODO: Change here
}

func (c *Coordinator) DivideBucket() {
	// TODO: If need to storage distributively
}

func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// scanForFiles:Add files into MTask list via prefix and suffix
