import { useEffect, useMemo, useState } from "react";
import { getSchema } from "@tiptap/core";
import { useAuth } from "../../auth/AuthProvider.jsx";
import { extensions } from "./editor.js";
import { ManuscriptRoom } from "./collaboration.js";
import ManuscriptEditor from "./ManuscriptEditor.jsx";
import Comments from "./Comments.jsx";
export default function CollaborativeManuscript({
  worldId,
  sourceWorldId,
  onRoom,
  ...props
}) {
  const auth = useAuth(),
    schema = useMemo(() => getSchema(extensions), []);
  const [room, setRoom] = useState(null),
    [state, setState] = useState({ ready: false }),
    [editor, setEditor] = useState(null),
    [commentsVersion, setCommentsVersion] = useState(0);
  useEffect(() => {
    if (!auth) return;
    const live = new ManuscriptRoom({
      worldId,
      node: props.node,
      userId: auth.user.id,
      schema,
      onState: setState,
      onComments: () => setCommentsVersion((v) => v + 1),
    });
    setRoom(live);
    onRoom?.(live);
    return () => {
      onRoom?.(null);
      live.destroy();
    };
  }, [worldId, props.node.id, auth?.user.id]);
  useEffect(() => {
    if (
      !editor ||
      !state.ready ||
      !["owner", "administrator", "editor"].includes(state.role) ||
      !sourceWorldId ||
      sourceWorldId === worldId
    )
      return;
    let tr = editor.state.tr;
    editor.state.doc.descendants((node, pos) => {
      for (const mark of node.marks) {
        if (
          mark.type.name === "worldReference" &&
          mark.attrs.worldId === sourceWorldId
        )
          tr = tr
            .removeMark(pos, pos + node.nodeSize, mark.type)
            .addMark(
              pos,
              pos + node.nodeSize,
              mark.type.create({ ...mark.attrs, worldId }),
            );
      }
    });
    if (tr.docChanged) editor.view.dispatch(tr);
  }, [editor, state.ready, state.role, worldId, sourceWorldId]);
  if (!auth) return <ManuscriptEditor {...props} />;
  const editable = ["owner", "administrator", "editor"].includes(state.role);
  return (
    <>
      <div className="collaboration-status" role="status">
        {state.error ||
          (!state.ready && editable
            ? "Preparing shared manuscript…"
            : state.connected
              ? state.pending
                ? "Synchronizing edits…"
                : "Live · all edits synchronized"
              : "Reconnecting · local edits retained")}
      </div>
      {state.ready ? (
        <ManuscriptEditor
          {...props}
          key={props.node.id + ":live"}
          collaboration={room?.doc}
          collaborationRoom={room}
          editable={editable}
          onEditor={(value) => {
            setEditor(value);
            props.onEditor(value);
          }}
        />
      ) : (
        !editable && (
          <ManuscriptEditor
            {...props}
            editable={false}
            onChange={() => {}}
            onBlur={() => {}}
          />
        )
      )}
      {state.ready && (
        <Comments
          node={props.node}
          editor={editor}
          room={room}
          role={state.role}
          version={commentsVersion}
        />
      )}
    </>
  );
}
