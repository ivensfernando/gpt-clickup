# Exemplos de requisições JSON

Abaixo estão modelos de payloads e respostas para o endpoint `/gpt-clickup`,
mostrando como pedir mapa de workspaces, listar tarefas em linguagem natural e
criar tarefas usando `list_path` em vez de IDs explícitos.

## Mapa de workspaces
**Request**
```json
{
  "prompt": "Quais são os meus workspaces?"
}
```

**Response**
```json
{
  "workspace_map": [
    {
      "id": "ws_123",
      "name": "Personal",
      "spaces": [
        {
          "id": "sp_marketing",
          "name": "Marketing",
          "lists": [
            {"id": "lst_blog", "name": "Blog"},
            {"id": "lst_social", "name": "Redes Sociais"}
          ],
          "folders": []
        }
      ]
    }
  ]
}
```

## Listar tarefas abertas de um workspace
**Request** (linguagem natural; não envia IDs)
```json
{
  "prompt": "Liste as tarefas abertas do workspace Personal"
}
```

**Response**
```json
{
  "workspace": {"id": "ws_123", "name": "Personal"},
  "open_only": true,
  "tasks": [
    {
      "id": "tsk_01",
      "name": "Planejar calendário do blog",
      "status": "em andamento",
      "priority": 2,
      "list_id": "lst_blog",
      "list_name": "Blog",
      "space_name": "Marketing",
      "workspace_name": "Personal"
    },
    {
      "id": "tsk_02",
      "name": "Brief de campanha de lançamento",
      "status": "a fazer",
      "priority": 1,
      "list_id": "lst_social",
      "list_name": "Redes Sociais",
      "space_name": "Marketing",
      "workspace_name": "Personal"
    }
  ]
}
```

## Criar uma tarefa usando caminho de lista (list_path)
**Request**
```json
{
  "prompt": "Criar tarefa para revisar landing page",
  "list_path": ["Personal", "Marketing", "Blog"],
  "force_sync": false
}
```

**Response**
```json
{
  "planner_messages": [
    {"role": "system", "content": "Você é um planejador especializado em ClickUp..."},
    {"role": "user", "content": "Pergunta original: Criar tarefa para revisar landing page\nContexto conhecido (JSON): ..."}
  ],
  "planner_raw": "{\"task\":{\"name\":\"Revisar landing page\",\"description\":\"Melhorar copy e verificar formulários\",\"status\":\"backlog\",\"priority\":2},\"explanation\":\"Planejado pela IA\"}",
  "planner": {
    "task": {
      "name": "Revisar landing page",
      "description": "Melhorar copy e verificar formulários",
      "status": "backlog",
      "priority": 2
    },
    "explanation": "Planejado pela IA"
  },
  "list_id": "lst_blog",
  "created_task": {
    "id": "tsk_03",
    "name": "Revisar landing page",
    "status": "backlog",
    "priority": 2,
    "list_id": "lst_blog"
  }
}