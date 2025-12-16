from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse
from fastapi.middleware.cors import CORSMiddleware
import httpx
import json
import uvicorn
import os

app = FastAPI()

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

def load_tools():
    base_dir = os.path.dirname(os.path.abspath(__file__))
    path = os.path.join(base_dir, "tools.json")
    with open(path) as f:
        return json.load(f)

TOOLS = load_tools()

def find_tool(tool_id: str):
    for category in TOOLS.values():
        for t in category:
            if t["id"] == tool_id:
                return t
    return None

@app.get("/stream")
async def stream(tool: str, target: str):
    tool_info = find_tool(tool)
    if not tool_info:
        raise HTTPException(400, "Unknown tool")

    if tool_info["type"] == "wasm":
        go_url = "http://127.0.0.1:9000/run-wasm"
        payload = {"module": tool_info["module"], "target": target}
    elif tool_info["type"] == "system":
        go_url = "http://127.0.0.1:9000/run-system"
        cmd = tool_info["cmd"].replace("{TARGET}", target)
        payload = {"cmd": cmd}
    else:
        raise HTTPException(400, "Unsupported tool type")

    async def stream_gen():
        # Use a new client for every request to ensure clean state
        async with httpx.AsyncClient(timeout=60.0) as client:
            try:
                # 1. Prepare Data Manually
                json_str = json.dumps(payload)
                json_bytes = json_str.encode("utf-8")
                
                print(f"🐍 PYTHON DEBUG: Sending {len(json_bytes)} bytes to Go: {json_str}")

                # 2. Build Request Explicitly (Bypassing client.stream helper)
                request = client.build_request(
                    "POST",
                    go_url,
                    content=json_bytes,
                    headers={
                        "Content-Type": "application/json",
                        "Content-Length": str(len(json_bytes)) # Force Content-Length
                    }
                )

                # 3. Send and Stream Response
                response = await client.send(request, stream=True)

                async for chunk in response.aiter_text():
                    if chunk.strip():
                        yield chunk
                
                # Close the response stream
                await response.aclose()
                yield "data: DONE\n\n"

            except Exception as e:
                print(f"🐍 PYTHON ERROR: {e}")
                yield f"data: ERROR: {str(e)}\n\n"

    return StreamingResponse(stream_gen(), media_type="text/event-stream")

@app.get("/tools")
def get_tools():
    return TOOLS

if __name__ == "__main__":
    print("🚀 Python orchestrator starting on http://127.0.0.1:8000")
    uvicorn.run(app, host="127.0.0.1", port=8000)