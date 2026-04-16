#!/usr/bin/env python3
"""
Pre-launch checklist for social media campaigns.

Usage:
    python3 scripts/prelaunch_check.py --project proseforge
    python3 scripts/prelaunch_check.py --project proseforge --campaign launch

Checks:
    - Credentials: All API keys present in .env
    - Site: Website returns 200 OK
    - Profiles: API connections work for each platform
    - Campaign: Campaign JSON exists and is valid
"""

import argparse
import json
import sys
from pathlib import Path

try:
    import requests
except ImportError:
    print("Error: requests not installed. Run: pip install requests")
    sys.exit(1)

SCRIPT_DIR = Path(__file__).parent
ROOT_DIR = SCRIPT_DIR.parent
DEFAULT_PROJECT = 'proseforge'


def log(msg: str, level: str = 'info'):
    """Print formatted log message."""
    prefix = {
        'info': '\033[34m[INFO]\033[0m',
        'ok': '\033[32m[OK]\033[0m',
        'warn': '\033[33m[WARN]\033[0m',
        'error': '\033[31m[ERROR]\033[0m',
        'check': '\033[36m[CHECK]\033[0m',
    }.get(level, '')
    print(f"{prefix} {msg}")


def load_env(project: str) -> dict[str, str]:
    """Load environment variables from project .env file."""
    env_path = ROOT_DIR / 'projects' / project / '.env'
    env_vars = {}

    if not env_path.exists():
        return env_vars

    for line in env_path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith('#'):
            continue
        if '=' in line:
            key, _, value = line.partition('=')
            env_vars[key.strip()] = value.strip()

    return env_vars


def check_credentials(project: str) -> tuple[bool, list[str]]:
    """Check that all required API credentials are present."""
    env = load_env(project)
    issues = []

    # X/Twitter credentials
    x_required = ['X_API_KEY', 'X_API_SECRET', 'X_ACCESS_TOKEN', 'X_ACCESS_SECRET']
    x_missing = [k for k in x_required if not env.get(k)]
    if x_missing:
        issues.append(f"X/Twitter: Missing {', '.join(x_missing)}")

    # Facebook credentials
    fb_required = ['FACEBOOK_PAGE_ID', 'FACEBOOK_ACCESS_TOKEN']
    fb_missing = [k for k in fb_required if not env.get(k)]
    if fb_missing:
        issues.append(f"Facebook: Missing {', '.join(fb_missing)}")

    # Instagram credentials
    ig_user_id = env.get('INSTAGRAM_USER_ID')
    ig_token = env.get('INSTAGRAM_ACCESS_TOKEN') or env.get('FACEBOOK_ACCESS_TOKEN')
    if not ig_user_id:
        issues.append("Instagram: Missing INSTAGRAM_USER_ID")
    if not ig_token:
        issues.append("Instagram: Missing access token")

    # YouTube (yutu CLI handles its own auth)
    yt_channel = env.get('YOUTUBE_CHANNEL_ID')
    if not yt_channel:
        issues.append("YouTube: Missing YOUTUBE_CHANNEL_ID")

    return len(issues) == 0, issues


def check_site(url: str) -> tuple[bool, str]:
    """Check that the website returns 200 OK."""
    try:
        response = requests.get(url, timeout=10, allow_redirects=True)
        if response.status_code == 200:
            return True, f"Site returns 200 OK ({len(response.content)} bytes)"
        else:
            return False, f"Site returns {response.status_code}"
    except requests.RequestException as e:
        return False, f"Site unreachable: {e}"


def check_x_api(project: str) -> tuple[bool, str]:
    """Check X/Twitter API connection."""
    try:
        import tweepy
    except ImportError:
        return False, "tweepy not installed"

    env = load_env(project)

    try:
        auth = tweepy.OAuth1UserHandler(
            env.get('X_API_KEY'),
            env.get('X_API_SECRET'),
            env.get('X_ACCESS_TOKEN'),
            env.get('X_ACCESS_SECRET')
        )
        api = tweepy.API(auth)
        user = api.verify_credentials()
        return True, f"Connected as @{user.screen_name}"
    except Exception as e:
        return False, f"API error: {e}"


def check_facebook_api(project: str) -> tuple[bool, str]:
    """Check Facebook API connection."""
    env = load_env(project)

    page_id = env.get('FACEBOOK_PAGE_ID')
    access_token = env.get('FACEBOOK_ACCESS_TOKEN')

    if not page_id or not access_token:
        return False, "Missing credentials"

    try:
        url = f"https://graph.facebook.com/v18.0/{page_id}"
        params = {'fields': 'name', 'access_token': access_token}
        response = requests.get(url, params=params, timeout=10)
        response.raise_for_status()
        data = response.json()
        return True, f"Connected to page: {data.get('name')}"
    except Exception as e:
        return False, f"API error: {e}"


def check_instagram_api(project: str) -> tuple[bool, str]:
    """Check Instagram API connection."""
    env = load_env(project)

    ig_user_id = env.get('INSTAGRAM_USER_ID')
    access_token = env.get('INSTAGRAM_ACCESS_TOKEN') or env.get('FACEBOOK_ACCESS_TOKEN')

    if not ig_user_id or not access_token:
        return False, "Missing credentials"

    try:
        url = f"https://graph.facebook.com/v18.0/{ig_user_id}"
        params = {'fields': 'username', 'access_token': access_token}
        response = requests.get(url, params=params, timeout=10)
        response.raise_for_status()
        data = response.json()
        return True, f"Connected as @{data.get('username')}"
    except Exception as e:
        return False, f"API error: {e}"


def check_campaign(project: str, campaign: str) -> tuple[bool, list[str]]:
    """Check campaign file exists and is valid."""
    campaign_path = ROOT_DIR / 'projects' / project / 'campaigns' / f'{campaign}.json'
    issues = []

    if not campaign_path.exists():
        return False, [f"Campaign file not found: {campaign_path}"]

    try:
        data = json.loads(campaign_path.read_text())
    except json.JSONDecodeError as e:
        return False, [f"Invalid JSON: {e}"]

    posts = data.get('posts', [])
    if not posts:
        issues.append("No posts in campaign")

    draft_count = 0
    for post in posts:
        if not post.get('id'):
            issues.append(f"Post missing UUID")
        if post.get('status') == 'draft':
            draft_count += 1
            # Check content
            has_content = any([
                post.get('twitter'),
                post.get('facebook'),
                post.get('instagram_caption')
            ])
            if not has_content:
                issues.append(f"Post {post.get('id', '?')[:8]} has no content")

    if draft_count == 0:
        issues.append("No draft posts ready to publish")

    return len(issues) == 0, issues


def run_checks(project: str, campaign: str = None, site_url: str = None) -> bool:
    """Run all pre-launch checks."""
    print(f"\n{'=' * 60}")
    print(f"Pre-Launch Checklist: {project}")
    print(f"{'=' * 60}\n")

    all_passed = True

    # 1. Credentials
    log("Checking credentials...", 'check')
    passed, issues = check_credentials(project)
    if passed:
        log("All credentials present", 'ok')
    else:
        all_passed = False
        for issue in issues:
            log(issue, 'error')

    # 2. Site check
    if site_url:
        print()
        log(f"Checking site: {site_url}", 'check')
        passed, msg = check_site(site_url)
        if passed:
            log(msg, 'ok')
        else:
            all_passed = False
            log(msg, 'error')

    # 3. API connections
    print()
    log("Checking API connections...", 'check')

    # X/Twitter
    passed, msg = check_x_api(project)
    if passed:
        log(f"X/Twitter: {msg}", 'ok')
    else:
        all_passed = False
        log(f"X/Twitter: {msg}", 'error')

    # Facebook
    passed, msg = check_facebook_api(project)
    if passed:
        log(f"Facebook: {msg}", 'ok')
    else:
        all_passed = False
        log(f"Facebook: {msg}", 'error')

    # Instagram
    passed, msg = check_instagram_api(project)
    if passed:
        log(f"Instagram: {msg}", 'ok')
    else:
        all_passed = False
        log(f"Instagram: {msg}", 'error')

    # 4. Campaign check
    if campaign:
        print()
        log(f"Checking campaign: {campaign}", 'check')
        passed, issues = check_campaign(project, campaign)
        if passed:
            log("Campaign file valid", 'ok')
        else:
            all_passed = False
            for issue in issues:
                log(issue, 'error')

    # Summary
    print(f"\n{'=' * 60}")
    if all_passed:
        log("All checks passed! Ready for launch.", 'ok')
    else:
        log("Some checks failed. Please fix issues before launch.", 'error')
    print(f"{'=' * 60}\n")

    return all_passed


def main():
    parser = argparse.ArgumentParser(
        description='Pre-launch checklist for social media campaigns',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  %(prog)s --project proseforge
  %(prog)s --project proseforge --campaign launch
  %(prog)s --project proseforge --site https://proseforge.ai
        '''
    )
    parser.add_argument('--project', '-p', default=DEFAULT_PROJECT,
                        help=f'Project name (default: {DEFAULT_PROJECT})')
    parser.add_argument('--campaign', '-c',
                        help='Campaign name to check')
    parser.add_argument('--site', metavar='URL',
                        help='Site URL to check (e.g., https://proseforge.ai)')

    args = parser.parse_args()

    # Default site URL for proseforge
    site_url = args.site
    if not site_url and args.project == 'proseforge':
        site_url = 'https://www.proseforge.ai'

    success = run_checks(args.project, args.campaign, site_url)
    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()
