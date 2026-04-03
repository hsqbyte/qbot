---
name: weather
description: 获取指定城市的天气状态。用户询问天气、气温、是否下雨等场景使用。
---

# Weather

查询城市天气信息。

## 执行方式

```bash
python3 scripts/execute.py '{"city": "北京"}'
```

## 示例

| 用户说 | 参数 |
|--------|------|
| 北京天气怎么样 | `{"city": "北京"}` |
| 上海下雨了吗 | `{"city": "上海"}` |
