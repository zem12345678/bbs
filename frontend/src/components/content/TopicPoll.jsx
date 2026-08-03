import React from "react";
import { Check, ListChecks } from "lucide-react";

import { bbsApi } from "../../api";
import { pollChoicePercent } from "../../lib/topicPoll";

export default function TopicPoll({ auth, initialPoll, topicId }) {
  const [poll, setPoll] = React.useState(initialPoll || null);
  const [selected, setSelected] = React.useState(() => selectedIndexes(initialPoll));
  const [state, setState] = React.useState({ submitting: false, error: "" });

  React.useEffect(() => {
    setPoll(initialPoll || null);
    setSelected(selectedIndexes(initialPoll));
    setState({ submitting: false, error: "" });
  }, [initialPoll, topicId]);

  if (!poll) return null;
  const hasVoted = Boolean(poll.has_voted ?? poll.hasVoted);
  const expired = Boolean(poll.expired);
  const multiple = Boolean(poll.multiple);
  const totalVoters = Number(poll.total_voters ?? poll.totalVoters) || 0;
  const choices = Array.isArray(poll.choices) ? poll.choices : [];
  const showResults = hasVoted || expired;

  function toggleChoice(index) {
    if (hasVoted || expired || state.submitting) return;
    if (!auth?.accessToken) {
      setState((current) => ({ ...current, error: "请先登录后参与投票。" }));
      return;
    }
    setState((current) => ({ ...current, error: "" }));
    setSelected((current) => {
      if (!multiple) return new Set([index]);
      const next = new Set(current);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }

  async function submitVote() {
    if (!auth?.accessToken || selected.size === 0 || hasVoted || expired) return;
    setState({ submitting: true, error: "" });
    try {
      const data = await bbsApi.voteTopicPoll(topicId, { choices: [...selected].sort((a, b) => a - b) }, auth.accessToken);
      const nextPoll = data?.poll || null;
      if (nextPoll) {
        setPoll(nextPoll);
        setSelected(selectedIndexes(nextPoll));
      }
      setState({ submitting: false, error: "" });
    } catch (error) {
      setState({ submitting: false, error: error.message || "投票提交失败" });
    }
  }

  return (
    <section className="topic-poll" aria-label="主题投票">
      <header>
        <span><ListChecks size={18} aria-hidden="true" />投票</span>
        <small>{expired ? "已截止" : hasVoted ? "已投票" : multiple ? "多选" : "单选"}</small>
      </header>
      <div className="topic-poll__options">
        {choices.map((choice, position) => {
          const index = Number(choice?.index ?? position);
          const active = selected.has(index) || Boolean(choice?.selected);
          const percent = pollChoicePercent(choice?.votes, totalVoters);
          return (
            <button
              className={active ? "is-selected" : ""}
              disabled={hasVoted || expired || state.submitting}
              key={`${index}-${choice?.text || position}`}
              type="button"
              onClick={() => toggleChoice(index)}
            >
              {showResults && <span className="topic-poll__bar" style={{ width: `${percent}%` }} />}
              <strong>{choice?.text}</strong>
              {active && <Check size={16} aria-hidden="true" />}
              {showResults && <em>{percent}% · {Number(choice?.votes) || 0} 票</em>}
            </button>
          );
        })}
      </div>
      <footer>
        <span>{totalVoters} 人参与{poll.expires_at ? ` · ${formatPollDeadline(poll.expires_at)}` : ""}</span>
        {!hasVoted && !expired && (
          <button disabled={state.submitting || selected.size === 0 || !auth?.accessToken} type="button" onClick={submitVote}>
            {state.submitting ? "提交中..." : "提交投票"}
          </button>
        )}
      </footer>
      {state.error && <p className="form-error">{state.error}</p>}
    </section>
  );
}

function selectedIndexes(poll) {
  const indexes = (poll?.choices || []).filter((choice) => choice?.selected).map((choice) => Number(choice.index));
  return new Set(indexes);
}

function formatPollDeadline(value) {
  const date = new Date(Number(value));
  if (Number.isNaN(date.getTime())) return "";
  return `截止 ${date.toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" })}`;
}
