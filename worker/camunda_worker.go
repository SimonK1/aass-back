package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// Camunda REST endpoint and worker config
const (
	camundaURL   = "http://localhost:8082/engine-rest"
	workerID     = "wac-go-worker"
	lockDuration = 600000            // in milliseconds
	maxTasks     = 5
	pollInterval = 5 * time.Second
)

// Variable represents a process variable in Camunda
type Variable struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// ExternalTask represents a Camunda external task
type ExternalTask struct {
	ID        string              `json:"id"`
	TopicName string              `json:"topicName"`
	Variables map[string]Variable `json:"variables"`
}

// Procedure matches your JSON structure for procedureData
type Procedure struct {
	Id          string  `json:"id"`
	Description string  `json:"description"`
	Patient     string  `json:"patient"`
	Price       float64 `json:"price"`
	VisitType   string  `json:"visitType"`
	Payer       string  `json:"payer"`
	AmbulanceId string  `json:"ambulanceId"`
	Timestamp   string  `json:"timestamp"`
}

// fetchAndLock polls Camunda for external tasks in the given topics
func fetchAndLock(topics []map[string]interface{}) ([]ExternalTask, error) {
	payload := map[string]interface{}{
		"workerId":    workerID,
		"maxTasks":    maxTasks,
		"usePriority": true,
		"topics":      topics,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal fetch payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(),
		"POST", camundaURL+"/external-task/fetchAndLock", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("create fetch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch request error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch returned %d: %s", resp.StatusCode, string(data))
	}

	var tasks []ExternalTask
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("decode fetch response: %w", err)
	}
	return tasks, nil
}

// completeTask completes the external task with the given variables
func completeTask(taskID string, variables map[string]interface{}) error {
	cv := make(map[string]map[string]interface{}, len(variables))
	for k, v := range variables {
		cv[k] = map[string]interface{}{"value": v, "type": "String"}
	}
	payload := map[string]interface{}{
		"workerId":  workerID,
		"variables": cv,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal complete payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(),
		"POST", camundaURL+"/external-task/"+taskID+"/complete", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create complete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("complete request error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("complete returned %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

// handleSave logs and completes the Save Performance Record task
func handleSave(t ExternalTask) {
	raw, ok := t.Variables["procedureData"]
	if !ok || raw.Value == nil {
		log.Printf("[Save] Missing procedureData for task %s", t.ID)
		return
	}
	var p Procedure
	if err := json.Unmarshal([]byte(raw.Value.(string)), &p); err != nil {
		log.Printf("[Save] Invalid JSON for task %s: %v", t.ID, err)
		return
	}
	log.Printf("[Save] Procedure ID: %s, Patient: %s", p.Id, p.Patient)
	if err := completeTask(t.ID, map[string]interface{}{"procedureId": p.Id}); err != nil {
		log.Printf("[Save] Failed to complete task %s: %v", t.ID, err)
	}
}

// handleValidate always marks data as valid (someVar="value1")
func handleValidate(t ExternalTask) {
	log.Printf("[Validate] Forcing valid data for task %s", t.ID)
	if err := completeTask(t.ID, map[string]interface{}{"someVar": "value1"}); err != nil {
		log.Printf("[Validate] Failed to complete task %s: %v", t.ID, err)
	}
}

// handleBilling auto-completes the Update Billing task
func handleBilling(t ExternalTask) {
	raw, ok := t.Variables["procedureId"]
	if !ok || raw.Value == nil {
		log.Printf("[Billing] Missing procedureId for task %s", t.ID)
	} else {
		log.Printf("[Billing] Updating billing for procedure %s (task %s)", raw.Value.(string), t.ID)
	}
	if err := completeTask(t.ID, nil); err != nil {
		log.Printf("[Billing] Failed to complete task %s: %v", t.ID, err)
	}
}

// handleNotify auto-completes the Notify Department task
func handleNotify(t ExternalTask) {
	raw, ok := t.Variables["procedureId"]
	if !ok || raw.Value == nil {
		log.Printf("[Notify] Missing procedureId for task %s", t.ID)
	} else {
		log.Printf("[Notify] Notifying department for procedure %s (task %s)", raw.Value.(string), t.ID)
	}
	if err := completeTask(t.ID, nil); err != nil {
		log.Printf("[Notify] Failed to complete task %s: %v", t.ID, err)
	}
}

// UserTask represents a Camunda user task for auto-completion
type UserTask struct {
	Id                string                 `json:"id"`
	Name              string                 `json:"name"`
	TaskDefinitionKey string                 `json:"taskDefinitionKey"`
	ProcessInstanceId string                 `json:"processInstanceId"`
	Variables         map[string]interface{} `json:"variables"`
}

// fetchUserTasks fetches all user tasks by definition key
func fetchUserTasks(defKey string) ([]UserTask, error) {
	url := fmt.Sprintf("%s/task?taskDefinitionKey=%s", camundaURL, defKey)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET /task returned %d: %s", resp.StatusCode, body)
	}
	var tasks []UserTask
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// completeUserTask completes the given user task with optional variables
func completeUserTask(taskID string, variables map[string]interface{}) error {
	payload := map[string]map[string]map[string]interface{}{"variables": {}}
	for k, v := range variables {
		payload["variables"][k] = map[string]interface{}{"value": v, "type": "String"}
	}
	b, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/task/%s/complete", camundaURL, taskID)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("complete user task returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

func main() {
	log.Println("🚀 Worker started")

	// External‐task polling
	go func() {
		topics := []map[string]interface{}{
			{"topicName": "taskTopic1", "lockDuration": lockDuration},
			{"topicName": "taskTopic2", "lockDuration": lockDuration},
			{"topicName": "taskTopic3", "lockDuration": lockDuration},
			{"topicName": "taskTopic4", "lockDuration": lockDuration},
		}
		for {
			tasks, err := fetchAndLock(topics)
			if err != nil {
				log.Printf("Fetch error: %v", err)
				time.Sleep(pollInterval)
				continue
			}
			if len(tasks) == 0 {
				time.Sleep(pollInterval)
				continue
			}
			for _, t := range tasks {
				switch t.TopicName {
				case "taskTopic1":
					handleSave(t)
				case "taskTopic2":
					handleValidate(t)
				case "taskTopic3":
					handleBilling(t)
				case "taskTopic4":
					handleNotify(t)
				default:
					log.Printf("No handler for topic %s", t.TopicName)
				}
			}
		}
	}()

	// Auto‐approve "Approve Submission" user tasks
	go func() {
		for {
			userTasks, err := fetchUserTasks("ApproveSubmission")
			if err != nil {
				log.Printf("Error fetching user tasks: %v", err)
				time.Sleep(pollInterval)
				continue
			}
			for _, ut := range userTasks {
				log.Printf("Auto‐approving user task %s (proc %s)", ut.Id, ut.ProcessInstanceId)
				if err := completeUserTask(ut.Id, map[string]interface{}{"approvedBy": "auto-bot"}); err != nil {
					log.Printf("Failed to complete user task %s: %v", ut.Id, err)
				}
			}
			time.Sleep(pollInterval)
		}
	}()

	// Block forever
	select {}
}
