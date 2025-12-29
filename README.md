🕶️ Go-Smith: The AI Agent Orchestrator
======================================

> _"Never send a human to do a machine's job."_

<div align="center">
<img src="go-smith.png" alt="Go-Smith Banner" width="45%", alt="sa">
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

*   `main.go`: The entry point. Initializes the registry and starts the server.
    
*   `orchestrator.go`: The "Brain". Handles HTTP requests, performs validation, and routes tasks.
    
*   `models/`: Contains the **Shared DTOs** (Request/Response structs).
    
*   `config/agents.json`: The registry file defining available agents, their endpoints, and schemas.
    
*   `test\_agents/`: Example agents written in different languages (Python, C++, Go) to demonstrate polyglot capabilities.


🚀 Getting Started
------------------

### Prerequisites

*   Go (1.21 or higher)
    
*   Python (Optional, for running the Python agent)
    
*   g++ / C++ Compiler (Optional, for running the C++ agent)
    

### Step 1: Clone the Repository

```bash
git clone https://github.com/uslanozan/Go-Smith.git
cd Go-Smith
```

### Step 2: Spin Up the Agents

Start the test agents in separate terminals to simulate a distributed environment:

**Python Finance Agent (Port 8001):**

```bash
cd test_agents/python_finance_agent
pip install fastapi uvicorn
python finance_agent.py
```

**C++ Math Agent (Port 8084):** _(Requires compilation first)_

```bash
cd test_agents/cpp_math_agent
# Compilation command may vary by OS. Example:
g++ -o math_agent main.cpp 
./math_agent
```

### Step 3: Start Go-Smith

Run the orchestrator from the root directory:

```bash
go mod tidy
go run .
```

Go-Smith is now online at http://localhost:8080




🧪 Usage Examples
-----------------

You can test the system using Postman or cURL.

### 1\. Finance Analysis (Python Agent)

**Request (POST http://localhost:8080/run\_task):**

```json
{
    "agent_name": "finance_analysis",
    "arguments": {
        "currency": "BTC"
    }
}
```

**Response:**

```json
{
    "status": "success",
    "data": {
        "symbol": "BTC",
        "price": 65432.10,
        "source": "Simulated Finance API"
    }
}
```

### 2\. Heavy Math Calculation (C++ Agent)

**Request (POST http://localhost:8080/run\_task):**

```json
{
    "agent_name": "heavy_math_calculation",
    "arguments": {
        "number": 12
    }
}
```

**Response:**

```json
{
    "original_number": 12,
    "square": 144,
    "cube": 1728,
    "processed_by": "C++ Heavy Math Agent"
}
```