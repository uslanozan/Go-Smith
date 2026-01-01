🕶️ Go-Smith: The AI Agent Orchestrator
======================================

[![Go Version](https://img.shields.io/github/go-mod/go-version/uslanozan/Go-Smith)](https://github.com/uslanozan/Go-Smith)
[![License](https://img.shields.io/github/license/uslanozan/Go-Smith)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](http://makeapullrequest.com)

> *"Never send a human to do a machine's job."*

<div align="center">
  <img src="go-smith-v2.png" alt="Go-Smith Banner" width="100%">
</div>

**Go-Smith** is a high-performance, dynamic **orchestration engine** designed to manage AI Agents written in various programming languages (Python, C++, Java, Go) under a centralized brain. Acting like the "Agent" management system in The Matrix, it breaks down complex tasks, routes them to the appropriate specialized agent, and delivers the results through a unified interface.

🚀 **Mission:** To render language-agnostic (polyglot) AI microservices manageable and scalable using a single standard (**Shared DTO**) and a robust **Dynamic Schema Validation** infrastructure.

🔥 Key Features
---------------

*   **🌍 Polyglot Architecture:** Whether it's Python, C++, Go, Java, or Node.js— Go-Smith can manage any agent that speaks HTTP.
    
*   **🧠 Dynamic Schema Validation:** Dynamically loads agent capabilities defined in task\_schema.json. It validates incoming requests against these schemas, catching invalid data at the gate before it reaches the agents.
    
*   **🤝 Shared DTO (Data Transfer Object):** The entire system (Orchestrator and Agents) adheres to a standardized data contract. Regardless of the implementation language, every component speaks the same JSON dialect.
    
*   **⚡ High-Performance Go Core:** Built on **Go's (Golang)** legendary speed and concurrency model. Capable of handling and routing thousands of agent tasks simultaneously.
    
*   **🔌 Plug & Play:** Adding a new agent is as simple as adding a single entry to config/agents.json. No core code changes are required.


📂 Project Structure
--------------------

The system follows a modular, microservice-ready structure:

```bash
Go-Smith
├───config
├───examples
│   └───simple_backend_demo/
├───models
├───schemas
├───scripts
├───secrets
├───test_agents
├───agent.go
├───main.go
└───orchestrator.go
```

*   `main.go`: The entry point. Initializes the registry and starts the server.
    
*   `orchestrator.go`: The "Brain". Handles HTTP requests, performs validation, and routes tasks.

*   `examples/`: Demo implementations (Gateway, Clients)
    
*   `models/`: Contains the **Shared DTOs** (Request/Response structs).
    
*   `config/agents.json`: The registry file defining available agents, their endpoints, and schemas.
    
*   `test_agents/`: Example agents written in different languages (Python, C++, Go) to demonstrate polyglot capabilities.


🛠️ Development Workflow: Shared DTOs
--------------------

One of Go-Smith's strongest features is its **Single Source of Truth** architecture. The system relies on a Shared DTO (Data Transfer Object) structure that enables seamless **Polyglot** (multi-language) support.

*   **For Non-Go Agents (Python, C++, etc.):** Agents strictly follow the JSON contract defined in schemas/task\_schema.json. As long as they speak JSON, the implementation language doesn't matter.
    
*   **For Go Agents:** You have the flexibility to rely solely on the **Shared DTO (JSON schema)**, just like non-Go agents. However, for a superior development experience, you can directly import models/task\_model.go. This allows you to share the exact same structs with the Orchestrator, ensuring **100% type safety** and eliminating code duplication.
    

### 🔄 Modifying the Task Structure

If you need to change the data payload structure (e.g., adding a new field to the task request), **you do not edit JSON files manually.**

1.  **Modify the Go Struct:** Edit the struct definitions in models/task\_model.go.
    
2.  **Auto-Generate Schema:** Run the helper script to update the JSON schema automatically.

```bash
go run scripts/generate_schema.go
```


🚀 Getting Started
------------------

### Prerequisites

*   Ollama

*   Go (1.24 or higher)
    
*   Python (**Optional**, for running the Python agent)
    
*   g++ / C++ Compiler (**Optional**, for running the C++ agent)


### Step 0: Install Ollama and a Tool Calling Model

Go-Smith relies on a local LLM to understand user intents and route tasks. I heavily recommend **Qwen 2.5** (or Llama 3.1) for its superior tool-calling capabilities.

1.  **Download Ollama** from [ollama.com](https://ollama.com).
2.  **Pull the Model:** Open your terminal and run the following command to download and serve the model:

**Note:** Ensure Ollama is running in the background (http://localhost:11434) before starting Go-Smith.

### Step 1: Clone the Repository

```bash
git clone https://github.com/uslanozan/Go-Smith.git
cd Go-Smith
```

### Step 2: Configuration

Create your environment configuration file from the example provided. This file configures the database, LLM settings, and API keys.

```bash
cp .env.example .env
```

_(You can keep the default settings for the demo, or edit .env to match your custom setup.)_

### Step 3: Spin Up the Agents

Start the test agents in separate terminals to simulate a distributed environment:

**Async Test Agent (Port 8083):**

```bash
cd test_agents/async_test_agent
go run async_test_agent.go
```

### Step 4: Start Go-Smith

Run the orchestrator from the root directory:

```bash
go mod tidy
go run .
```

Go-Smith is now online at http://localhost:8080


🧪 Usage Examples (Go-Smith + Agents)
-----------------

You can test the system by mimicking the Backend and Ollama response using Postman or cURL.

### 1\. Mock PDF Converter (Go Agent)

**Request (POST http://localhost:8080/run_task):**

```json
{
    "agent_name": "pdf_converter",
    "arguments": {
        "file_name": "annual_report_2024.txt"
    }
}
```

**Response:**

```json
{
    "task_id": "go-task-75111",
    "status": "running"
}
```

### 2\. Check Task Status

Once an async task involves a waiting period, you can poll the status endpoint provided in the previous response.

**Request (GET http://localhost:8080/task_status/go-task-75111):**

(No body required)

**Response:**

```json
{
    "task_id": "go-task-75111",
    "status": "running",
    "result": "PDF conversion started..."
}
```

**Response (Completed):**

```json
{
    "task_id": "go-task-75111",
    "status": "completed",
    "result": {
        "download_url": "https://cdn.gosmith.local/annual_report_2024.txt.pdf",
        "message": "Conversion successful"
    }
}
```

### 3\. Stop a Running Task

If the task is taking too long or is no longer needed, you can send a stop signal. The Orchestrator routes this to the correct agent's stop endpoint.

**Request (POST http://localhost:8080/task_stop/go-task-90210):**

(No body required)

**Response:**

```json
{
    "status": "cancelled",
    "message": "Task execution stopped by user request."
}
```


🌟 Optional: Run the Full Stack (Go-Smith + Ollama + Gateway + DB + Agents)
-----------------

Want to see how Go-Smith fits into a real-world application? We provided a **complete backend simulation** in the `examples/` folder. unlike the manual tests above, this setup involves the **Real LLM (Ollama)** making decisions.

This demo includes:
* **Ollama(Port 11434):** An instruct LLM (e.g., Qwen 2.5) that acts as the "Brain", generating tool calls based on natural language. LLM decides whether the prompt is a chat prompt or task prompt (if it's a task, decides which tool will be using)
* **Gateway (Port 8000):** A Go Fiber/HTTP server simulating a real backend.
* **Database:** A zero-config **MySQL** database to store chat history and users.
* **Auth:** Simple API Key authentication.

### 1. Start the Ollama (Terminal 1)

```bash
ollama run qwen2.5:3b-instruct
```

### 2. Start the Gateway (Terminal 2)
Instead of calling the Orchestrator directly, let's talk to the Gateway.

```bash
cd examples/simple_backend_demo
go run . # Gateway is listening on http://localhost:8000
```

### 3. Start the C++ Agent (Terminal 3)

To demonstrate Go-Smith's ability to manage low-level languages, let's spin up the C++ Math Agent.

_Note: You need a C++ compiler (g++ or clang)._

```bash
cd test_agents/cpp_math_agent
g++ -o math_agent main.cpp
./math_agent # C++ Agent is listening on http://localhost:8084
```

### 4. Start the Go-Smith (Terminal 4)

Run the orchestrator from the root directory.

```bash
go run .
```


### 5\. End-to-End Test

Now, send a natural language prompt to the **Gateway**. The Gateway will ask Ollama, Ollama will choose the tool, and Go-Smith will route it to the C++ Agent.

**Request (POST http://localhost:8000/chat):**

```bash
curl -X POST http://localhost:8000/chat \
     -H "Authorization: Bearer demo-token-123" \
     -H "Content-Type: application/json" \
     -d '{
           "prompt": "Calculate the square of 12 using the heavy math agent."
         }'
```

**What happens in the background?**

1.  **Gateway (Port 8000)** receives the user prompt *"Calculate square of 12..."*.

2.  **Context Construction:** The Gateway pulls the tool definitions (schemas) from **Go-Smith** and constructs a request for **Ollama**.

3.  **LLM Inference:** **Qwen2.5:3b-instruct** analyzes the prompt against the available tools. It detects a mathematical intent and generates a structured **Tool Call JSON**:
    ```json
    {
    "name": "heavy_math_calculation",
    "arguments": {
        "number": 12
      }
    }
    ```
    

4.  **Orchestration:** The Gateway forwards this structured request to **Go-Smith (8080)**.

5.  **Routing & Validation:** Go-Smith validates the arguments against the strict schema defined in `agents.json`. Once validated, it routes the payload to the running **C++ Agent (8084)**.

6.  **Execution:** The C++ binary performs the computation (squaring the number) and returns the result.

7.  **Response:** The Gateway persists the interaction (User Prompt + Assistant Response) to **MySQL** and delivers the final answer to the client.


🔮 Future Work & Roadmap
-----------------

I am actively working on making Go-Smith more robust and scalable. Here is what's coming next:

*   **🐳 Docker & Docker Compose Support:** A one-click setup to spin up the Orchestrator, Gateway, MySQL, and all Agents in isolated containers.
    
*   **📡 gRPC Support:** Implementing gRPC for high-performance, low-latency communication between the Orchestrator and Agents.
    
*   **📊 Web Dashboard:** A visual interface to monitor active agents, running tasks, and system health in real-time.
    
*   **🔐 Advanced Auth & RBAC:** Adding Role-Based Access Control for the Gateway to manage different user tiers.
    
*   **🧠 Vector Database Integration:** Native support for vector stores (like Qdrant or Milvus) to give agents long-term memory.


📄 LICENSE
-----------------
Distributed under the **MIT License**.