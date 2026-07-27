export function hasSearchResults(posts = [], users = []) {
  return posts.length > 0 || users.length > 0;
}
