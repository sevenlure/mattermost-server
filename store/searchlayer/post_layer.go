// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package searchlayer

import (
	"github.com/mattermost/mattermost-server/mlog"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/store"
)

type SearchPostStore struct {
	store.PostStore
	rootStore *SearchStore
}

func (s SearchPostStore) indexPost(post *model.Post) {
	if s.rootStore.searchEngine.GetActiveEngine().IsIndexingEnabled() {
		go (func() {
			channel, chanErr := s.rootStore.Channel().GetForPost(post.Id)
			if chanErr != nil {
				mlog.Error("Couldn't get channel for post for SearchEngine indexing.", mlog.String("channel_id", post.ChannelId), mlog.String("post_id", post.Id))
				return
			}
			if err := s.rootStore.searchEngine.GetActiveEngine().IndexPost(post, channel.TeamId); err != nil {
				mlog.Error("Encountered error indexing post", mlog.String("post_id", post.Id), mlog.Err(err))
			}
		})()
	}
}

func (s SearchPostStore) deletePostIndex(post *model.Post) {
	if s.rootStore.searchEngine.GetActiveEngine().IsIndexingEnabled() {
		go (func() {
			if err := s.rootStore.searchEngine.GetActiveEngine().DeletePost(post); err != nil {
				mlog.Error("Encountered error deleting post", mlog.String("post_id", post.Id), mlog.Err(err))
			}
		})()
	}
}

func (s SearchPostStore) Update(newPost, oldPost *model.Post) (*model.Post, *model.AppError) {
	post, err := s.PostStore.Update(newPost, oldPost)

	if err == nil {
		s.indexPost(post)
	}
	return post, err
}

func (s SearchPostStore) Save(post *model.Post) (*model.Post, *model.AppError) {
	npost, err := s.PostStore.Save(post)

	if err == nil {
		s.indexPost(npost)
	}
	return npost, err
}

func (s SearchPostStore) Delete(postId string, date int64, deletedByID string) *model.AppError {
	err := s.PostStore.Delete(postId, date, deletedByID)

	if err == nil {
		postList, err2 := s.PostStore.Get(postId, true)
		if err2 != nil {
			s.deletePostIndex(postList.Posts[postList.Order[0]])
		}
	}
	return err
}

func (s SearchPostStore) SearchPostsInTeamForUser(paramsList []*model.SearchParams, userId, teamId string, isOrSearch, includeDeletedChannels bool, page, perPage int) (*model.PostSearchResults, *model.AppError) {
	// We only allow the user to search in channels they are a member of.
	userChannels, err := s.rootStore.Channel().GetChannels(teamId, userId, includeDeletedChannels)
	if err != nil {
		mlog.Error("error getting channel for user", mlog.Err(err))
		return nil, err
	}

	postIds, matches, err := s.rootStore.searchEngine.GetActiveEngine().SearchPosts(userChannels, paramsList, page, perPage)
	if err != nil {
		return nil, err
	}

	// Get the posts
	postList := model.NewPostList()
	if len(postIds) > 0 {
		posts, err := s.PostStore.GetPostsByIds(postIds)
		if err != nil {
			return nil, err
		}
		for _, p := range posts {
			if p.DeleteAt == 0 {
				postList.AddPost(p)
				postList.AddOrder(p.Id)
			}
		}
	}

	return model.MakePostSearchResults(postList, matches), nil
}