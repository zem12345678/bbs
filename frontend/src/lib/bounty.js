export function publishedBountyMinimum({ isQuestion = true, status = 0, bountyScore = 0 } = {}) {
  const score = nonNegativeNumber(bountyScore);
  return isQuestion && Number(status) === 2 && score > 0 ? score : 0;
}

export function clampBountyScore(value, minimum = 0) {
  return Math.max(nonNegativeNumber(value), nonNegativeNumber(minimum));
}

function nonNegativeNumber(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) {
    return 0;
  }
  return number;
}
