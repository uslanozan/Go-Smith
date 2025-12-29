import time
import uuid
import threading
from typing import Optional, Dict, Any
from fastapi import FastAPI, HTTPException
from enum import Enum

# ARTIK MODELLERİ BURADAN ÇEKİYORUZ 👇
from models import OrchestratorTaskRequest, TaskStartResponse, TaskStatusResponse

app = FastAPI()

# In-Memory Database
tasks_db: Dict[str, Dict[str, Any]] = {}

# ---------------------------------------------------------
# İŞ MANTIĞI (Worker)
# ---------------------------------------------------------
def fetch_crypto_price(task_id: str, currency: str):
    try:
        tasks_db[task_id]["status"] = TaskStatus.RUNNING
        
        # Simülasyon
        time.sleep(2) 
        
        price_mock = 0
        curr = str(currency).upper() # Gelen veri Any olabileceği için string'e cast
        
        if curr == "BTC":
            price_mock = 95432.50
        elif curr == "ETH":
            price_mock = 2750.20
        elif curr == "USD":
            price_mock = 34.15
            
        result_data = {
            "currency": curr,
            "price": price_mock,
            "timestamp": time.time(),
            "source": "FinanceAgent-v1"
        }

        tasks_db[task_id]["status"] = TaskStatus.COMPLETED
        tasks_db[task_id]["result"] = result_data

    except Exception as e:
        tasks_db[task_id]["status"] = TaskStatus.FAILED
        tasks_db[task_id]["error"] = str(e)

# ---------------------------------------------------------
# ENDPOINTLER
# ---------------------------------------------------------

@app.post("/execute", response_model=TaskStartResponse)
async def execute_task(request: OrchestratorTaskRequest):
    """
    Görev isteğini karşılar.
    """
    # 1. Argümanları al (request.arguments artık Go'dan gelen RawMessage/Dict)
    # Python tarafında bu bir dict olarak gelir.
    args = request.arguments if isinstance(request.arguments, dict) else {}
    currency = args.get("currency", "BTC")
    
    task_id = str(uuid.uuid4())
    
    tasks_db[task_id] = {
        "status": TaskStatus.PENDING,
        "result": None,
        "error": None
    }
    
    thread = threading.Thread(target=fetch_crypto_price, args=(task_id, currency))
    thread.start()
    
    # TaskStartResponse modelini kullanarak dönüyoruz
    return TaskStartResponse(
        task_id=task_id,
        status=TaskStatus.PENDING.value
    )

@app.get("/task_status/{task_id}", response_model=TaskStatusResponse)
async def get_task_status(task_id: str):
    """
    Durum sorgulama.
    """
    if task_id not in tasks_db:
        raise HTTPException(status_code=404, detail="Task not found")
    
    task_data = tasks_db[task_id]
    
    # TaskStatusResponse modelini kullanarak dönüyoruz
    return TaskStatusResponse(
        task_id=task_id,
        status=task_data["status"].value, # Enum -> String dönüşümü
        result=task_data["result"],
        error=task_data["error"]
    )

if __name__ == "__main__":
    import uvicorn
    print("🚀 Finance Agent (Shared DTO) running on port 8001")
    uvicorn.run(app, host="0.0.0.0", port=8001)