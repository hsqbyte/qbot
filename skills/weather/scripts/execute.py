#!/usr/bin/env python3
import sys
import json
import random

def info():
    # Return JSON schema when called with `info`
    print(json.dumps({
        "name": "get_weather_external",
        "description": "获取指定城市的天气状态",
        "parameters_schema": json.dumps({
            "type": "object",
            "properties": {
                "city": {"type": "string", "description": "城市"}
            },
            "required": ["city"]
        })
    }))

def execute(args_json):
    # Execute the skill logic when called with `execute`
    try:
        args = json.loads(args_json)
        city = args.get("city", "未知城市")
        weather = random.choice(["晴朗", "下雨", "多云", "打雷"])
        temp = random.randint(15, 35)
        print(f"第三方天气服务返回: {city} 的天气是 {weather}，当前温度 {temp} 度")
    except Exception as e:
        print(f"Error parsing args: {e}")

if __name__ == "__main__":
    args_json = sys.stdin.read().strip()
    if not args_json:
        sys.exit(1)
    execute(args_json)
