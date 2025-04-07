package v1

import (
    "context"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    v1pb "github.com/usememos/memos/proto/gen/api/v1"
    "github.com/usememos/memos/store"
)

func (s *APIV1Service) CreateMemoComment(ctx context.Context, request *v1pb.CreateMemoCommentRequest) (*v1pb.Memo, error) {
    memoUID, err := ExtractMemoUIDFromName(request.Parent)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid parent memo name: %v", err)
    }

    parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get parent memo: %v", err)
    }
    if parentMemo == nil {
        return nil, status.Errorf(codes.NotFound, "parent memo not found")
    }

    user, err := s.GetCurrentUser(ctx)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
    }

    comment := &store.Memo{
        UID:        shortuuid.New(),
        CreatorID:  user.ID,
        Content:    request.Comment.Content,
        Visibility: store.Private,
        ParentID:   &parentMemo.ID,
    }
    if err := s.Store.CreateMemo(ctx, comment); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create comment: %v", err)
    }

    if _, err := s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
        MemoID:        comment.ID,
        RelatedMemoID: parentMemo.ID,
        Type:          store.MemoRelationComment,
    }); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create memo relation: %v", err)
    }

    commentMessage, err := s.convertMemoFromStore(ctx, comment)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to convert comment: %v", err)
    }

    return commentMessage, nil
}

func (s *APIV1Service) ListMemoComments(ctx context.Context, request *v1pb.ListMemoCommentsRequest) (*v1pb.ListMemoCommentsResponse, error) {
    memoUID, err := ExtractMemoUIDFromName(request.Parent)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid parent memo name: %v", err)
    }

    parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to get parent memo: %v", err)
    }
    if parentMemo == nil {
        return nil, status.Errorf(codes.NotFound, "parent memo not found")
    }

    commentType := store.MemoRelationComment
    relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
        RelatedMemoID: &parentMemo.ID,
        Type:          &commentType,
    })
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to list memo comments: %v", err)
    }

    var comments []*v1pb.Memo
    for _, relation := range relations {
        commentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &relation.MemoID})
        if err != nil {
            return nil, status.Errorf(codes.Internal, "failed to get comment memo: %v", err)
        }
        if commentMemo != nil {
            commentMessage, err := s.convertMemoFromStore(ctx, commentMemo)
            if err != nil {
                return nil, status.Errorf(codes.Internal, "failed to convert comment memo: %v", err)
            }
            comments = append(comments, commentMessage)
        }
    }

    return &v1pb.ListMemoCommentsResponse{
        Comments: comments,
    }, nil
}