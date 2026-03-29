export default function minutesAgo(date) {
  const now = new Date();
  const diffMs = now - new Date(date);
  return Math.floor(diffMs / 1000 / 60);
}