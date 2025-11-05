# Gollama-the-Orchestrator
LLM orchestrator for agents that built with Go


-Run Ollama

-Execute Commands (3 CLI)
    -1) go run .
    -2) cd .\test_agents\ && go run fake_slack_agent.go
    -3) cd .\llm_gateway\ && go run main.go

-POST API
    http://localhost:8000/chat and body: { "prompt": "Send a message to channel C0123ABC with text 'Bu test Ollama'dan geldi!'" }