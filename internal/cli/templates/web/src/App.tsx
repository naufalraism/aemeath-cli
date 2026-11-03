import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { ApiError, createClient } from "./api/client";
import { createHooks } from "./api/hooks";
import { postApiTasksBodySchema } from "./api/schemas";

// ./api/* is generated from the Go routes by "aemeath dev --ts-out web/src/api"
// (or "aemeath ts --out web/src/api"); do not edit those files by hand.
const client = createClient({ baseUrl: "" });
const { useGetApiTasks, usePostApiTasks, useDeleteApiTasksById } = createHooks(client);

function describe(error: unknown): string {
  if (error instanceof ApiError && error.errors.length > 0) {
    return error.errors.map((e) => `${e.field} ${e.message}`).join(", ");
  }
  return error instanceof Error ? error.message : "Something went wrong";
}

export default function App() {
  const queryClient = useQueryClient();
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["getApiTasks"] });

  const tasks = useGetApiTasks();
  const create = usePostApiTasks({ onSuccess: refresh });
  const remove = useDeleteApiTasksById({ onSuccess: refresh });

  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);

  function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);

    // The same rules the Go server enforces, checked in the browser first.
    const body = { name: name.trim() };
    const check = postApiTasksBodySchema.safeParse(body);
    if (!check.success) {
      setError(check.error.issues[0]?.message ?? "Invalid input");
      return;
    }
    create.mutate({ body }, { onSuccess: () => setName(""), onError: (e) => setError(describe(e)) });
  }

  return (
    <main>
      <h1>Tasks</h1>

      <form onSubmit={submit}>
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="What needs doing?" aria-label="Task" />
        <button type="submit" disabled={create.isPending}>
          Add
        </button>
      </form>
      {error && <p role="alert">{error}</p>}

      {tasks.isPending && <p>Loading…</p>}
      {tasks.isError && <p role="alert">Could not load the tasks: {describe(tasks.error)}</p>}
      <ul>
        {(tasks.data ?? []).map((task) => (
          <li key={task.id}>
            <span>{task.name}</span>
            <button onClick={() => remove.mutate({ path: { id: task.id } })} disabled={remove.isPending}>
              Delete
            </button>
          </li>
        ))}
      </ul>
    </main>
  );
}
