#include <iostream>
#include <thread>
#include <chrono>
#include <map>
#include <mutex>
#include <string>

#include "httplib.h"
#include "json.hpp"

using json = nlohmann::json;
using namespace std;

map<string, json> tasks_db;
mutex db_mutex;

void worker_task(string task_id, int number) {
    {
        lock_guard<mutex> lock(db_mutex);
        tasks_db[task_id]["status"] = "running";
    }

    this_thread::sleep_for(chrono::seconds(3));
    
    int square = number * number;

    {
        lock_guard<mutex> lock(db_mutex);
        tasks_db[task_id]["status"] = "completed";
        tasks_db[task_id]["result"] = {
            {"input", number},
            {"square", square},
            {"message", "Hello from C++"}
        };
    }
    cout << "[C++] Task Tamamlandi: " << task_id << endl;
}

int main() {
    httplib::Server svr;

    cout << "🚀 C++ Math Agent calisiyor -> Port: 8084" << endl;

    // 1. EXECUTE ENDPOINT (Gorev Baslat)
    svr.Post("/execute", [](const httplib::Request& req, httplib::Response& res) {
        try {
            auto body = json::parse(req.body);

            if (!body.contains("arguments")) {
                res.status = 400;
                res.set_content("Missing arguments", "text/plain");
                return;
            }

            auto now = chrono::system_clock::now().time_since_epoch().count();
            string task_id = "cpp-" + to_string(now);

            int number = 0;
            if (body["arguments"].contains("number")) {
                number = body["arguments"]["number"].get<int>();
            }

            {
                lock_guard<mutex> lock(db_mutex);
                tasks_db[task_id] = {
                    {"task_id", task_id},
                    {"status", "pending"}
                };
            }

            thread t(worker_task, task_id, number);
            t.detach();

            json response = {
                {"task_id", task_id},
                {"status", "pending"}
            };
            res.set_content(response.dump(), "application/json");

        } catch (...) {
            res.status = 400;
            res.set_content("Invalid JSON", "text/plain");
        }
    });

    svr.Get(R"(/task_status/(.*))", [](const httplib::Request& req, httplib::Response& res) {
        string task_id = req.matches[1];

        lock_guard<mutex> lock(db_mutex);
        if (tasks_db.count(task_id)) {
            res.set_content(tasks_db[task_id].dump(), "application/json");
        } else {
            res.status = 404;
            res.set_content("Task not found", "text/plain");
        }
    });

    svr.listen("0.0.0.0", 8084);
}