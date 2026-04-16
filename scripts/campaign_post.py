#!/usr/bin/env python3
"""
Campaign orchestrator for coordinated multi-platform social media posting.

Usage:
    python3 scripts/campaign_post.py --project proseforge --campaign launch --list
    python3 scripts/campaign_post.py --project proseforge --post UUID
    python3 scripts/campaign_post.py --project proseforge --retract UUID
    python3 scripts/campaign_post.py --project proseforge --post UUID --dry-run

Campaign JSON format (projects/{project}/campaigns/{campaign}.json):
{
  "campaign": "launch",
  "posts": [
    {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "description": "Main launch announcement",
      "status": "draft",
      "created_at": "2026-01-14T15:00:00Z",
      "posted_at": null,
      "platform_ids": {},
      "twitter": "Tweet content here...",
      "facebook": "Facebook post content...",
      "instagram_caption": "Instagram caption...",
      "instagram_image": "https://example.com/image.png"
    }
  ]
}

Status lifecycle: draft -> posted -> retracted
"""

import argparse
import json
import sys
from datetime import datetime
from pathlib import Path

# Import platform posting functions
SCRIPT_DIR = Path(__file__).parent
ROOT_DIR = SCRIPT_DIR.parent
sys.path.insert(0, str(SCRIPT_DIR))

from x_profile import post_tweet, delete_tweet
from facebook_page import post_to_page, delete_post as fb_delete_post
from instagram_profile import post_image as ig_post_image, delete_post as ig_delete_post

DEFAULT_PROJECT = 'proseforge'


def log(msg: str, level: str = 'info'):
    """Print formatted log message."""
    prefix = {
        'info': '\033[34m[INFO]\033[0m',
        'ok': '\033[32m[OK]\033[0m',
        'warn': '\033[33m[WARN]\033[0m',
        'error': '\033[31m[ERROR]\033[0m',
    }.get(level, '')
    print(f"{prefix} {msg}")


def load_campaign(project: str, campaign: str) -> dict | None:
    """Load campaign JSON file."""
    campaign_path = ROOT_DIR / 'projects' / project / 'campaigns' / f'{campaign}.json'

    if not campaign_path.exists():
        log(f"Campaign file not found: {campaign_path}", 'error')
        return None

    try:
        return json.loads(campaign_path.read_text())
    except json.JSONDecodeError as e:
        log(f"Invalid JSON in campaign file: {e}", 'error')
        return None


def save_campaign(project: str, campaign: str, data: dict) -> bool:
    """Save campaign JSON file."""
    campaign_path = ROOT_DIR / 'projects' / project / 'campaigns' / f'{campaign}.json'

    try:
        campaign_path.write_text(json.dumps(data, indent=2) + '\n')
        return True
    except Exception as e:
        log(f"Failed to save campaign: {e}", 'error')
        return False


def find_post_by_id(campaign_data: dict, post_id: str) -> dict | None:
    """Find a post by UUID in campaign data."""
    for post in campaign_data.get('posts', []):
        if post.get('id') == post_id or post.get('id', '').startswith(post_id):
            return post
    return None


def list_posts(project: str, campaign: str) -> bool:
    """List all posts in a campaign."""
    data = load_campaign(project, campaign)
    if not data:
        return False

    posts = data.get('posts', [])
    if not posts:
        log("No posts in campaign", 'info')
        return True

    print(f"\nCampaign: {data.get('campaign', campaign)}")
    print("-" * 60)

    for post in posts:
        status = post.get('status', 'unknown')
        status_color = {
            'draft': '\033[33m',      # yellow
            'posted': '\033[32m',     # green
            'retracted': '\033[31m',  # red
        }.get(status, '')

        print(f"\n  ID: {post.get('id')}")
        print(f"  Description: {post.get('description', '(no description)')}")
        print(f"  Status: {status_color}{status}\033[0m")

        if post.get('posted_at'):
            print(f"  Posted: {post.get('posted_at')}")

        platform_ids = post.get('platform_ids', {})
        if platform_ids:
            print(f"  Platform IDs: {platform_ids}")

        # Show content preview
        if post.get('twitter'):
            print(f"  Twitter: {post['twitter'][:50]}...")
        if post.get('facebook'):
            print(f"  Facebook: {post['facebook'][:50]}...")
        if post.get('instagram_caption'):
            print(f"  Instagram: {post['instagram_caption'][:50]}...")

    print()
    return True


def post_campaign(project: str, campaign: str, post_id: str, dry_run: bool = False) -> bool:
    """Post a campaign post to all platforms."""
    data = load_campaign(project, campaign)
    if not data:
        return False

    post = find_post_by_id(data, post_id)
    if not post:
        log(f"Post not found with ID: {post_id}", 'error')
        return False

    if post.get('status') != 'draft':
        log(f"Post status is '{post.get('status')}' - can only post drafts", 'error')
        return False

    print(f"\nPosting: {post.get('description', post_id)}")
    print("-" * 60)

    platform_ids = {}
    success_count = 0
    total_platforms = 0

    # Post to Twitter/X
    if post.get('twitter'):
        total_platforms += 1
        print("\n[Twitter/X]")
        result = post_tweet(project, post['twitter'], dry_run)
        if result:
            platform_ids['twitter'] = result.get('tweet_id')
            success_count += 1

    # Post to Facebook
    if post.get('facebook'):
        total_platforms += 1
        print("\n[Facebook]")
        result = post_to_page(project, post['facebook'], dry_run=dry_run)
        if result:
            platform_ids['facebook'] = result.get('post_id')
            success_count += 1

    # Post to Instagram
    if post.get('instagram_caption') and post.get('instagram_image'):
        total_platforms += 1
        print("\n[Instagram]")
        result = ig_post_image(project, post['instagram_caption'], post['instagram_image'], dry_run)
        if result:
            platform_ids['instagram'] = result.get('media_id')
            success_count += 1

    if total_platforms == 0:
        log("No platform content found in post", 'warn')
        return False

    print(f"\n{'=' * 60}")
    log(f"Posted to {success_count}/{total_platforms} platforms", 'ok' if success_count == total_platforms else 'warn')

    if not dry_run and success_count > 0:
        # Update post status
        post['status'] = 'posted'
        post['posted_at'] = datetime.utcnow().isoformat() + 'Z'
        post['platform_ids'] = platform_ids

        if save_campaign(project, campaign, data):
            log(f"Campaign file updated", 'ok')
        else:
            log("Warning: Failed to update campaign file", 'warn')

    return success_count == total_platforms


def retract_campaign(project: str, campaign: str, post_id: str, dry_run: bool = False) -> bool:
    """Retract a campaign post from all platforms."""
    data = load_campaign(project, campaign)
    if not data:
        return False

    post = find_post_by_id(data, post_id)
    if not post:
        log(f"Post not found with ID: {post_id}", 'error')
        return False

    if post.get('status') != 'posted':
        log(f"Post status is '{post.get('status')}' - can only retract posted content", 'error')
        return False

    platform_ids = post.get('platform_ids', {})
    if not platform_ids:
        log("No platform IDs found - cannot retract", 'error')
        return False

    # Confirmation prompt
    if not dry_run:
        print(f"\nAbout to retract: {post.get('description', post_id)}")
        print(f"Platforms: {', '.join(platform_ids.keys())}")
        confirm = input("Delete from all platforms? [y/N] ").strip().lower()
        if confirm != 'y':
            log("Retraction cancelled", 'info')
            return False

    print(f"\nRetracting: {post.get('description', post_id)}")
    print("-" * 60)

    success_count = 0
    total_platforms = len(platform_ids)

    # Delete from Twitter/X
    if 'twitter' in platform_ids:
        print("\n[Twitter/X]")
        tweet_id = platform_ids['twitter']
        if tweet_id and tweet_id != 'posted':
            if delete_tweet(project, tweet_id, dry_run):
                success_count += 1
        else:
            log("No tweet ID available - cannot delete", 'warn')

    # Delete from Facebook
    if 'facebook' in platform_ids:
        print("\n[Facebook]")
        fb_id = platform_ids['facebook']
        if fb_id and fb_id != 'posted':
            if fb_delete_post(project, fb_id, dry_run):
                success_count += 1
        else:
            log("No Facebook post ID available - cannot delete", 'warn')
            log("Manual deletion required at https://facebook.com/ProseForge", 'info')

    # Delete from Instagram
    if 'instagram' in platform_ids:
        print("\n[Instagram]")
        ig_id = platform_ids['instagram']
        if ig_id and ig_id != 'posted':
            if ig_delete_post(project, ig_id, dry_run):
                success_count += 1
        else:
            log("No Instagram media ID available - cannot delete", 'warn')
            log("Manual deletion required in Instagram app", 'info')

    print(f"\n{'=' * 60}")
    log(f"Retracted from {success_count}/{total_platforms} platforms", 'ok' if success_count > 0 else 'warn')

    if not dry_run:
        # Update post status
        post['status'] = 'retracted'
        post['retracted_at'] = datetime.utcnow().isoformat() + 'Z'

        if save_campaign(project, campaign, data):
            log(f"Campaign file updated", 'ok')
        else:
            log("Warning: Failed to update campaign file", 'warn')

    return success_count > 0


def main():
    parser = argparse.ArgumentParser(
        description='Campaign orchestrator for multi-platform social media posting',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  %(prog)s --project proseforge --campaign launch --list
  %(prog)s --project proseforge --campaign launch --post UUID
  %(prog)s --project proseforge --campaign launch --retract UUID
  %(prog)s --project proseforge --campaign launch --post UUID --dry-run
        '''
    )
    parser.add_argument('--project', '-p', default=DEFAULT_PROJECT,
                        help=f'Project name (default: {DEFAULT_PROJECT})')
    parser.add_argument('--campaign', '-c', default='launch',
                        help='Campaign name (default: launch)')

    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument('--list', action='store_true', help='List all posts in campaign')
    action.add_argument('--post', metavar='UUID', help='Post a draft by UUID')
    action.add_argument('--retract', metavar='UUID', help='Retract a posted item by UUID')

    parser.add_argument('--dry-run', action='store_true', help='Preview without changes')

    args = parser.parse_args()

    if args.list:
        success = list_posts(args.project, args.campaign)
    elif args.post:
        success = post_campaign(args.project, args.campaign, args.post, args.dry_run)
    elif args.retract:
        success = retract_campaign(args.project, args.campaign, args.retract, args.dry_run)
    else:
        success = False

    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()
